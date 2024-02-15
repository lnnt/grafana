package expr

import (
	"embed"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/grafana/grafana-plugin-sdk-go/data/utils/jsoniter"
	"github.com/grafana/grafana-plugin-sdk-go/experimental/query"

	"github.com/grafana/grafana/pkg/apis/query/v0alpha1"
	"github.com/grafana/grafana/pkg/expr"
	"github.com/grafana/grafana/pkg/expr/classic"
	"github.com/grafana/grafana/pkg/expr/mathexp"
	"github.com/grafana/grafana/pkg/services/featuremgmt"
	"github.com/grafana/grafana/pkg/tsdb/legacydata"
)

type ExpressionQuery struct {
	RefID   string
	Command expr.Command
}

var _ query.TypedQueryReader[ExpressionQuery] = (*ExpressionQueryReader)(nil)

type ExpressionQueryReader struct {
	k8s      *v0alpha1.QueryTypeDefinitionList
	features featuremgmt.FeatureToggles
}

//go:embed query.json
var f embed.FS

func NewExpressionQueryReader(features featuremgmt.FeatureToggles) (*ExpressionQueryReader, error) {
	h := &ExpressionQueryReader{
		k8s:      &v0alpha1.QueryTypeDefinitionList{},
		features: features,
	}

	body, err := f.ReadFile("query.json")
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(body, h.k8s)
	if err != nil {
		return nil, err
	}

	field := ""
	for _, qt := range h.k8s.Items {
		if field == "" {
			field = qt.Spec.DiscriminatorField
		} else if qt.Spec.DiscriminatorField != "" {
			if qt.Spec.DiscriminatorField != field {
				return nil, fmt.Errorf("only one discriminator field allowed")
			}
		}
	}

	return h, nil
}

// QueryTypes implements query.TypedQueryHandler.
func (h *ExpressionQueryReader) QueryTypeDefinitionList() *v0alpha1.QueryTypeDefinitionList {
	return h.k8s
}

// ReadQuery implements query.TypedQueryHandler.
func (h *ExpressionQueryReader) ReadQuery(
	// Properties that have been parsed off the same node
	common query.CommonQueryProperties,
	// An iterator with context for the full node (include common values)
	iter *jsoniter.Iterator,
) (eq ExpressionQuery, err error) {
	referenceVar := ""
	eq.RefID = common.RefID
	qt := QueryType(common.QueryType)
	switch qt {
	case QueryTypeMath:
		q := &MathQuery{}
		err = iter.ReadVal(q)
		if err == nil {
			eq.Command, err = expr.NewMathCommand(common.RefID, q.Expression)
		}

	case QueryTypeReduce:
		var mapper mathexp.ReduceMapper = nil
		q := &ReduceQuery{}
		err = iter.ReadVal(q)
		if err == nil {
			referenceVar, err = getReferenceVar(q.Expression, common.RefID)
		}
		if err == nil && q.Settings != nil {
			switch q.Settings.Mode {
			case ReduceModeDrop:
				mapper = mathexp.DropNonNumber{}
			case ReduceModeReplace:
				if q.Settings.ReplaceWithValue == nil {
					err = fmt.Errorf("setting replaceWithValue must be specified when mode is '%s'", q.Settings.Mode)
				}
				mapper = mathexp.ReplaceNonNumberWithValue{Value: *q.Settings.ReplaceWithValue}
			default:
				err = fmt.Errorf("unsupported reduce mode")
			}
		}
		if err == nil {
			eq.Command, err = expr.NewReduceCommand(common.RefID,
				string(q.Reducer), referenceVar, mapper)
		}

	case QueryTypeResample:
		q := &ResampleQuery{}
		err = iter.ReadVal(q)
		if err == nil && common.TimeRange == nil {
			err = fmt.Errorf("missing time range in query")
		}
		if err == nil {
			referenceVar, err = getReferenceVar(q.Expression, common.RefID)
		}
		if err == nil {
			tr := legacydata.NewDataTimeRange(common.TimeRange.From, common.TimeRange.To)
			eq.Command, err = expr.NewResampleCommand(common.RefID,
				q.Window,
				referenceVar,
				q.Downsampler,
				q.Upsampler,
				expr.AbsoluteTimeRange{
					From: tr.GetFromAsTimeUTC(),
					To:   tr.GetToAsTimeUTC(),
				})
		}

	case QueryTypeClassic:
		q := &ClassicQuery{}
		err = iter.ReadVal(q)
		if err == nil {
			eq.Command, err = classic.NewConditionCmd(common.RefID, q.Conditions)
		}

	case QueryTypeThreshold:
		q := &ThresholdQuery{}
		err = iter.ReadVal(q)
		if err == nil {
			referenceVar, err = getReferenceVar(q.Expression, common.RefID)
		}
		if err == nil {
			// we only support one condition for now, we might want to turn this in to "OR" expressions later
			if len(q.Conditions) != 1 {
				return eq, fmt.Errorf("threshold expression requires exactly one condition")
			}
			firstCondition := q.Conditions[0]

			threshold, err := expr.NewThresholdCommand(common.RefID, referenceVar, firstCondition.Evaluator.Type, firstCondition.Evaluator.Params)
			if err != nil {
				return eq, fmt.Errorf("invalid condition: %w", err)
			}
			eq.Command = threshold

			if firstCondition.UnloadEvaluator != nil && h.features.IsEnabledGlobally(featuremgmt.FlagRecoveryThreshold) {
				unloading, err := expr.NewThresholdCommand(common.RefID, referenceVar, firstCondition.UnloadEvaluator.Type, firstCondition.UnloadEvaluator.Params)
				unloading.Invert = true
				if err != nil {
					return eq, fmt.Errorf("invalid unloadCondition: %w", err)
				}
				var d expr.Fingerprints
				if firstCondition.LoadedDimensions != nil {
					d, err = expr.FingerprintsFromFrame(firstCondition.LoadedDimensions)
					if err != nil {
						return eq, fmt.Errorf("failed to parse loaded dimensions: %w", err)
					}
				}
				eq.Command, err = expr.NewHysteresisCommand(common.RefID, referenceVar, *threshold, *unloading, d)
				if err != nil {
					return eq, err
				}
			}
		}

	default:
		err = fmt.Errorf("unknown query type")
	}
	return
}

func getReferenceVar(exp string, refId string) (string, error) {
	exp = strings.TrimPrefix(exp, "%")
	if exp == "" {
		return "", fmt.Errorf("no variable specified to reference for refId %v", refId)
	}
	return exp, nil
}
