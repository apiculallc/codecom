package move

import (
	"fmt"
	"os"
)

// ValidationError is one row-level validation failure.
type ValidationError struct {
	SessionID   string
	SessionFile string
	Message     string
}

// ValidationErrors is an aggregated batch validation failure.
type ValidationErrors struct {
	Items []ValidationError
}

func (e *ValidationErrors) Error() string {
	if len(e.Items) == 0 {
		return "move validation failed"
	}
	return fmt.Sprintf("move validation failed with %d error(s)", len(e.Items))
}

// ValidatePlan validates all selected rows and reports all failures before any write.
func ValidatePlan(plan Plan) error {
	var errs []ValidationError
	for _, item := range plan.Items {
		rowErrs := validateItem(plan, item)
		errs = append(errs, rowErrs...)
	}
	if len(errs) == 0 {
		return nil
	}
	return &ValidationErrors{Items: errs}
}

func validateItem(plan Plan, item PlanItem) []ValidationError {
	out := make([]ValidationError, 0, 4)
	if item.SessionID == "" {
		out = append(out, ValidationError{
			SessionID:   item.SessionID,
			SessionFile: item.SessionFile,
			Message:     "missing session id",
		})
	}
	if item.SessionFile == "" {
		out = append(out, ValidationError{
			SessionID:   item.SessionID,
			SessionFile: item.SessionFile,
			Message:     "missing session file",
		})
	}
	if item.OldCWD == "" || item.OldCWD == "." {
		out = append(out, ValidationError{
			SessionID:   item.SessionID,
			SessionFile: item.SessionFile,
			Message:     "missing source cwd",
		})
		return out
	}
	if !isUnderRoot(item.OldCWD, plan.SourceRoot) {
		out = append(out, ValidationError{
			SessionID:   item.SessionID,
			SessionFile: item.SessionFile,
			Message:     fmt.Sprintf("cwd %q is outside source root %q", item.OldCWD, plan.SourceRoot),
		})
		return out
	}
	expected, err := remapPath(plan.SourceRoot, plan.TargetRoot, item.OldCWD)
	if err != nil {
		out = append(out, ValidationError{
			SessionID:   item.SessionID,
			SessionFile: item.SessionFile,
			Message:     fmt.Sprintf("map path: %v", err),
		})
		return out
	}
	if item.NewCWD == "" {
		out = append(out, ValidationError{
			SessionID:   item.SessionID,
			SessionFile: item.SessionFile,
			Message:     "missing mapped target cwd",
		})
		return out
	}
	if expected != item.NewCWD {
		out = append(out, ValidationError{
			SessionID:   item.SessionID,
			SessionFile: item.SessionFile,
			Message:     fmt.Sprintf("mapped target mismatch: expected %q got %q", expected, item.NewCWD),
		})
	}
	info, err := os.Stat(item.NewCWD)
	if err != nil {
		out = append(out, ValidationError{
			SessionID:   item.SessionID,
			SessionFile: item.SessionFile,
			Message:     fmt.Sprintf("target path does not exist: %q", item.NewCWD),
		})
		return out
	}
	if !info.IsDir() {
		out = append(out, ValidationError{
			SessionID:   item.SessionID,
			SessionFile: item.SessionFile,
			Message:     fmt.Sprintf("target path is not a directory: %q", item.NewCWD),
		})
	}
	return out
}
