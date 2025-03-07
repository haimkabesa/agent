package stages

// This package is ported over from grafana/loki/clients/pkg/logentry/stages.
// We aim to port the stages in steps, to avoid introducing huge amounts of
// new code without being able to slowly review, examine and test them.

import (
	"os"
	"runtime"
	"time"

	"github.com/go-kit/log"
	"github.com/grafana/agent/component/common/loki"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"
	"gopkg.in/yaml.v2"
)

// TODO(@tpaschalis) Let's use this as the list of stages we need to port over.
const (
	StageTypeJSON         = "json"
	StageTypeLogfmt       = "logfmt"
	StageTypeRegex        = "regex"
	StageTypeReplace      = "replace"
	StageTypeMetric       = "metrics"
	StageTypeLabel        = "labels"
	StageTypeLabelDrop    = "labeldrop"
	StageTypeTimestamp    = "timestamp"
	StageTypeOutput       = "output"
	StageTypeDocker       = "docker"
	StageTypeCRI          = "cri"
	StageTypeMatch        = "match"
	StageTypeTemplate     = "template"
	StageTypePipeline     = "pipeline"
	StageTypeTenant       = "tenant"
	StageTypeDrop         = "drop"
	StageTypeLimit        = "limit"
	StageTypeMultiline    = "multiline"
	StageTypePack         = "pack"
	StageTypeLabelAllow   = "labelallow"
	StageTypeStaticLabels = "static_labels"
)

// Processor takes an existing set of labels, timestamp and log entry and returns either a possibly mutated
// timestamp and log entry
type Processor interface {
	Process(labels model.LabelSet, extracted map[string]interface{}, time *time.Time, entry *string)
	Name() string
}

type Entry struct {
	Extracted map[string]interface{}
	loki.Entry
}

// Stage can receive entries via an inbound channel and forward mutated entries to an outbound channel.
type Stage interface {
	Name() string
	Run(chan Entry) chan Entry
}

func (entry *Entry) copy() *Entry {
	out, err := yaml.Marshal(entry)
	if err != nil {
		return nil
	}

	var n *Entry
	err = yaml.Unmarshal(out, &n)
	if err != nil {
		return nil
	}

	return n
}

// stageProcessor Allow to transform a Processor (old synchronous pipeline stage) into an async Stage
type stageProcessor struct {
	Processor

	inspector *inspector
}

func (s stageProcessor) Run(in chan Entry) chan Entry {
	return RunWith(in, func(e Entry) Entry {
		var before *Entry

		if Inspect {
			before = e.copy()
		}

		s.Process(e.Labels, e.Extracted, &e.Timestamp, &e.Line)

		if Inspect {
			s.inspector.inspect(s.Processor.Name(), before, e)
		}

		return e
	})
}

func toStage(p Processor) Stage {
	return &stageProcessor{
		Processor: p,
		inspector: newInspector(os.Stderr, runtime.GOOS == "windows"),
	}
}

// New creates a new stage for the given type and configuration.
func New(logger log.Logger, jobName *string, cfg StageConfig, registerer prometheus.Registerer) (Stage, error) {
	var (
		s   Stage
		err error
	)
	switch {
	// case StageTypeDocker:
	// 	s, err = NewDocker(logger, registerer)
	// 	if err != nil {
	// 		return nil, err
	// 	}
	// case StageTypeCRI:
	// 	s, err = NewCRI(logger, registerer)
	// 	if err != nil {
	// 		return nil, err
	// 	}
	case cfg.JSONConfig != nil:
		s, err = newJSONStage(logger, cfg.JSONConfig)
		if err != nil {
			return nil, err
		}
	// case StageTypeLogfmt:
	// 	s, err = newLogfmtStage(logger, cfg)
	// 	if err != nil {
	// 		return nil, err
	// 	}
	// case StageTypeRegex:
	// 	s, err = newRegexStage(logger, cfg)
	// 	if err != nil {
	// 		return nil, err
	// 	}
	// case StageTypeMetric:
	// 	s, err = newMetricStage(logger, cfg, registerer)
	// 	if err != nil {
	// 		return nil, err
	// 	}
	case cfg.LabelsConfig != nil:
		s, err = newLabelStage(logger, *cfg.LabelsConfig)
		if err != nil {
			return nil, err
		}
	// case StageTypeLabelDrop:
	// 	s, err = newLabelDropStage(cfg)
	// 	if err != nil {
	// 		return nil, err
	// 	}
	// case StageTypeTimestamp:
	// 	s, err = newTimestampStage(logger, cfg)
	// 	if err != nil {
	// 		return nil, err
	// 	}
	// case StageTypeOutput:
	// 	s, err = newOutputStage(logger, cfg)
	// 	if err != nil {
	// 		return nil, err
	// 	}
	// case StageTypeMatch:
	// 	s, err = newMatcherStage(logger, jobName, cfg, registerer)
	// 	if err != nil {
	// 		return nil, err
	// 	}
	// case StageTypeTemplate:
	// 	s, err = newTemplateStage(logger, cfg)
	// 	if err != nil {
	// 		return nil, err
	// 	}
	// case StageTypeTenant:
	// 	s, err = newTenantStage(logger, cfg)
	// 	if err != nil {
	// 		return nil, err
	// 	}
	// case StageTypeReplace:
	// 	s, err = newReplaceStage(logger, cfg)
	// 	if err != nil {
	// 		return nil, err
	// 	}
	// case StageTypeDrop:
	// 	s, err = newDropStage(logger, cfg, registerer)
	// 	if err != nil {
	// 		return nil, err
	// 	}
	// case StageTypeLimit:
	// 	s, err = newLimitStage(logger, cfg, registerer)
	// 	if err != nil {
	// 		return nil, err
	// 	}
	// case StageTypeMultiline:
	// 	s, err = newMultilineStage(logger, cfg)
	// 	if err != nil {
	// 		return nil, err
	// 	}
	// case StageTypePack:
	// 	s, err = newPackStage(logger, cfg, registerer)
	// 	if err != nil {
	// 		return nil, err
	// 	}
	// case StageTypeLabelAllow:
	// 	s, err = newLabelAllowStage(cfg)
	// 	if err != nil {
	// 		return nil, err
	// 	}
	// case StageTypeStaticLabels:
	// 	s, err = newStaticLabelsStage(logger, cfg)
	// 	if err != nil {
	// 		return nil, err
	// 	}
	default:
		panic("unreacheable; should have decoded into one of the StageConfig fields")
	}
	return s, nil
}
