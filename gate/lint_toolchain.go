// Vision: LintToolchain bundles lint configuration shared by Lint and Format (QualityScope-style assembly).
package gate

// LintToolchain bundles lint configuration shared by [Lint] and [Format].
// Construct with [NewLintToolchain]; zero value is invalid at step entry.
type LintToolchain struct {
	configPath     string
	customGCLPath  string
	customLintSpec string
	lintToolSpec   string
	lintArgs       []string
	valid          bool
}

// NewLintToolchain assembles lint configuration for [Lint] and [Format].
// [LintOption] applies only here, not on step calls.
func NewLintToolchain(
	config LintConfigValue,
	tool LintToolValue,
	opts ...LintOption,
) (LintToolchain, error) {
	if err := validateLintInputs(config, tool); err != nil {
		return LintToolchain{}, err
	}
	cfg := lintConfig{}
	for _, opt := range opts {
		opt(&cfg)
	}
	if err := validateLintCustomBinary(cfg); err != nil {
		return LintToolchain{}, err
	}
	return LintToolchain{
		configPath:     config.path,
		customGCLPath:  cfg.customGCLPath,
		customLintSpec: cfg.customLintSpec,
		lintToolSpec:   tool.spec,
		lintArgs:       append([]string(nil), cfg.lintArgs...),
		valid:          true,
	}, nil
}

//nolint:gocritic // Opaque value token
func validateLintToolchain(lt LintToolchain) error {
	if !lt.valid {
		return newValidationError(
			"lint toolchain",
			"LintToolchain is required — pass gate.NewLintToolchain(...)",
			ErrInvalidOption,
		)
	}
	return nil
}

//nolint:gocritic // Opaque value token
func (lt LintToolchain) lintConfig() lintConfig {
	return lintConfig{
		customGCLPath:  lt.customGCLPath,
		customLintSpec: lt.customLintSpec,
		lintArgs:       append([]string(nil), lt.lintArgs...),
	}
}
