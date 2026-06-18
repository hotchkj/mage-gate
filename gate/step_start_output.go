// Vision: Standardized, per-step start lines from core step execution so all consumers get consistent progress text.
package gate

import (
	"sort"
	"strings"
)

const (
	stepLineLint             = "Lint"
	stepLineFormat           = "Format"
	stepLineDeadcode         = "Deadcode"
	stepLineMarkdownLint     = "Markdown lint"
	stepLineVet              = "Vet"
	stepLineCompile          = "Compile"
	stepLineTest             = "Test"
	stepLineCoveredTest      = "Covered Test"
	stepLineCoverage         = "Coverage"
	stepLineCrap             = "CRAP"
	stepLineDuration         = "Duration"
	stepLineMutationScan     = "Mutation Scan"
	stepLineMutationSites    = "Mutation Sites"
	stepLineMutationCoverage = "Mutation Coverage"
	stepLineMutationKills    = "Mutation Kills"
	stepLineSuffix           = "..."
	defaultTagSliceCap       = 4
)

func emitStepStart(runner CommandRunner, title, qualifier string) {
	emitter := runnerAsStepDisplay(runner)
	if emitter == nil || title == "" {
		return
	}
	line := title
	if qualifier != "" {
		line += " [" + qualifier + "]"
	}
	line += stepLineSuffix
	emitter.EmitStepStartLine(line)
}

func emitStepStartFromToken(emitter stepDisplay, title string) {
	if emitter == nil || title == "" {
		return
	}
	emitter.EmitStepStartLine(title + stepLineSuffix)
}

func parseGoTestTags(args []string) string {
	if len(args) == 0 {
		return ""
	}
	tags := make([]string, 0, len(args))
	for argIndex := 0; argIndex < len(args); argIndex++ {
		arg := strings.TrimSpace(args[argIndex])
		if arg == "" {
			continue
		}
		if strings.HasPrefix(arg, "-tags=") {
			tags = append(tags, splitAndNormalizeTags(strings.TrimPrefix(arg, "-tags="))...)
			continue
		}
		if arg == "-tags" && argIndex+1 < len(args) {
			argIndex++
			tags = append(tags, splitAndNormalizeTags(args[argIndex])...)
		}
	}
	return joinUniqueTags(tags)
}

func splitAndNormalizeTags(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == ' ' || r == '\t'
	})
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		t := strings.TrimSpace(p)
		if t != "" {
			out = append(out, t)
		}
	}
	return out
}

func joinUniqueTags(tags []string) string {
	if len(tags) == 0 {
		return ""
	}
	seen := make(map[string]struct{}, len(tags))
	uniq := make([]string, 0, len(tags))
	for _, t := range tags {
		if _, exists := seen[t]; exists {
			continue
		}
		seen[t] = struct{}{}
		uniq = append(uniq, t)
	}
	sort.Strings(uniq)
	return strings.Join(uniq, ",")
}

func qualifierForTest(cfg testConfig) string {
	if tags := parseGoTestTags(cfg.testArgs); tags != "" {
		return "tags=" + tags
	}
	return ""
}

func qualifierForCoveredTest(cfg testConfig, production QualityScope) string {
	tagParts := make([]string, 0, defaultTagSliceCap)
	tagParts = append(tagParts, qualityScopeTags(production)...)
	tagParts = append(tagParts, splitAndNormalizeTags(parseGoTestTags(cfg.testArgs))...)
	if tags := joinUniqueTags(tagParts); tags != "" {
		return "tags=" + tags
	}
	return ""
}
