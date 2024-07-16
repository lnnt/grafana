"""
This module returns the pipeline used for triggering a downstream pipeline for Grafana Enterprise.
"""

load(
    "scripts/drone/steps/lib.star",
    "enterprise_downstream_step",
)
load(
    "scripts/drone/utils/utils.star",
    "pipeline",
)

trigger = {
    "event": [
        "push",
    ],
    "branch": "main",
    "paths": {
        "exclude": [
            "*.md",
            "docs/**",
            "latest.json",
        ],
    },
    "repo": [
        "grafana/grafana",
    ],
}

def enterprise_downstream_pipeline(
    trigger=trigger,
    ver_mode="main",
    depends_on = [
        "main-build-e2e-publish",
        "main-integration-tests",
    ]
):
    environment = {"EDITION": "oss"}
    steps = [
        enterprise_downstream_step(ver_mode=ver_mode),
    ]
    return pipeline(
        name = "{}-trigger-downstream".format(ver_mode),
        trigger = trigger,
        services = [],
        steps = steps,
        depends_on = depends_on,
        environment = environment,
    )
