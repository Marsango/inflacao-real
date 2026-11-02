package scraperhealth

import (
	"errors"
	"regexp"
	"time"
	"unicode/utf8"
)

type Diagnostic struct {
	Severity string `json:"severity"`
	Category string `json:"category"`
	Message  string `json:"message"`
}
type Report struct {
	Version      string       `json:"schema_version"`
	ScraperID    string       `json:"scraper_id"`
	ShopID       string       `json:"shop_id"`
	RunID        string       `json:"run_id"`
	Name         string       `json:"scraper_name"`
	Started      time.Time    `json:"started_at"`
	Finished     time.Time    `json:"finished_at"`
	Observations int          `json:"observations"`
	Errors       int          `json:"errors"`
	Warnings     int          `json:"warnings"`
	Omitted      int          `json:"omitted"`
	Diagnostics  []Diagnostic `json:"diagnostics"`
}

var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

func (report Report) validate() error {
	if report.Version != "1.0" || !uuidPattern.MatchString(report.RunID) || report.ScraperID == "" || report.ShopID == "" {
		return errors.New("invalid identity")
	}
	if report.Name == "" || utf8.RuneCountInString(report.Name) > 100 || report.Started.IsZero() || report.Finished.Before(report.Started) {
		return errors.New("invalid run")
	}
	if report.Observations < 0 || report.Errors < 0 || report.Warnings < 0 || report.Omitted < 0 || len(report.Diagnostics) > 100 {
		return errors.New("invalid counts")
	}
	return report.validateDiagnostics()
}
func (report Report) validateDiagnostics() error {
	counts := map[string]int{"error": 0, "warning": 0}
	var err error
	for index := 0; index < len(report.Diagnostics) && err == nil; index++ {
		diagnostic := report.Diagnostics[index]
		err = diagnostic.validate()
		counts[diagnostic.Severity]++
	}
	if err != nil {
		return err
	}
	if counts["error"] > report.Errors || counts["warning"] > report.Warnings || report.Errors+report.Warnings != len(report.Diagnostics)+report.Omitted {
		return errors.New("inconsistent counts")
	}
	return nil
}
func (diagnostic Diagnostic) validate() error {
	if diagnostic.Severity != "error" && diagnostic.Severity != "warning" {
		return errors.New("invalid severity")
	}
	if diagnostic.Category == "" || len(diagnostic.Category) > 100 || diagnostic.Message == "" || utf8.RuneCountInString(diagnostic.Message) > 2000 {
		return errors.New("invalid diagnostic")
	}
	return nil
}
