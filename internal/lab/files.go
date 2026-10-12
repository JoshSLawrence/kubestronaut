package lab

import (
	"encoding/json"
	"io/fs"

	"github.com/JoshSLawrence/kubestronaut/internal/grade"
	"github.com/JoshSLawrence/kubestronaut/internal/scenario"
)

func fsReadFile(s *scenario.Scenario, name string) ([]byte, error) {
	return fs.ReadFile(s.Files, name)
}

func marshalReport(r grade.Report) ([]byte, error) {
	return json.MarshalIndent(r, "", "  ")
}
