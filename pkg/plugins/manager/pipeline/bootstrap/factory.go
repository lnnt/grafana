package bootstrap

import (
	"fmt"

	"github.com/grafana/grafana/pkg/plugins"
	"github.com/grafana/grafana/pkg/plugins/log"
)

type pluginFactoryFunc func(p plugins.FoundPlugin, pluginClass plugins.Class, sig plugins.Signature) (*plugins.Plugin, error)

// DefaultPluginFactory is the default plugin factory used by the Construct step of the Bootstrap stage.
type DefaultPluginFactory struct{}

// NewDefaultPluginFactory returns a new DefaultPluginFactory.
func NewDefaultPluginFactory() *DefaultPluginFactory {
	return &DefaultPluginFactory{}
}

func (f *DefaultPluginFactory) createPlugin(p plugins.FoundPlugin, class plugins.Class,
	sig plugins.Signature) (*plugins.Plugin, error) {

	plugin := &plugins.Plugin{
		JSONData:      p.JSONData,
		Class:         class,
		FS:            p.FS,
		Signature:     sig.Status,
		SignatureType: sig.Type,
		SignatureOrg:  sig.SigningOrg,
	}

	plugin.SetLogger(log.New(fmt.Sprintf("plugin.%s", plugin.ID)))

	return plugin, nil
}
