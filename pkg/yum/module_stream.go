package yum

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/ast"
)

// Better userfacing struct
type ModuleStream struct {
	Name    string
	Streams []Stream
}

type Stream struct {
	Name        string                 `yaml:"name"`
	Stream      StreamVersion          `yaml:"stream"`
	Version     string                 `yaml:"version"`
	Context     string                 `yaml:"context"`
	Arch        string                 `yaml:"arch"`
	Summary     string                 `yaml:"summary"`
	Description string                 `yaml:"description"`
	Artifacts   Artifacts              `yaml:"artifacts"`
	Profiles    map[string]RpmProfiles `yaml:"profiles"`
}

type StreamVersion string

// unmarshalStreamVersion ensures trailing zeros is preserved
// in cases are the stream value is a float like 5.30
func unmarshalStreamVersion(s *StreamVersion, data []byte) error {
	str := strings.TrimSpace(string(data))

	//Remove additional quotes when stream is represented as string
	if len(str) >= 2 && str[0] == '"' && str[len(str)-1] == '"' {
		str = str[1 : len(str)-1]
	}

	*s = StreamVersion(str)
	return nil
}

func (s StreamVersion) String() string {
	return string(s)
}

type RpmProfiles struct {
	Rpms []string `yaml:"rpms"`
}

type Artifacts struct {
	Rpms []string `yaml:"rpms"`
}

type ModuleMD struct {
	Document string `yaml:"document"`
	Version  int    `yaml:"version"`
	Data     Stream `yaml:"data"`
}

// ModuleMDs Returns the modulemd documents from the "modules" metadata in the given yum repository
func (r *Repository) ModuleMDs(ctx context.Context) ([]ModuleMD, int, error) {
	var modulesURL *string
	var err error
	var resp *http.Response
	var moduleMDs []ModuleMD

	if r.moduleMDs != nil {
		return r.moduleMDs, 200, nil
	}

	if _, _, err := r.Repomd(ctx); err != nil {
		return nil, 0, fmt.Errorf("error parsing repomd.xml: %w", err)
	}

	if modulesURL, err = r.getModulesURL(); err != nil {
		return nil, 0, fmt.Errorf("error parsing modules md URL: %w", err)
	}

	if modulesURL != nil {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, *modulesURL, nil)
		if err != nil {
			return nil, 0, fmt.Errorf("error creating request: %w", err)
		}

		if resp, err = r.settings.Client.Do(req); err != nil /* #nosec G704 */ {
			return nil, erroredStatusCode(resp), fmt.Errorf("GET error for file %v: %w", modulesURL, err)
		}
		defer resp.Body.Close()

		if moduleMDs, err = parseModuleMDs(resp.Body); err != nil {
			return nil, resp.StatusCode, fmt.Errorf("error parsing comps.xml: %w", err)
		}

		return moduleMDs, resp.StatusCode, nil
	}
	r.moduleMDs = moduleMDs
	return moduleMDs, 0, err
}

// parseModuleMDs moduleMDs contain multiple document types
// this breaks parsing into two parts:
// 1. use node to read the document type
// 2. if the document type is modulemd, fully decode the value
func parseModuleMDs(body io.ReadCloser) ([]ModuleMD, error) {
	moduleMDs := make([]ModuleMD, 0)

	reader, err := ExtractIfCompressed(body)
	if err != nil {
		return moduleMDs, fmt.Errorf("error extracting compressed streams: %w", err)
	}

	yaml.RegisterCustomUnmarshaler[StreamVersion](unmarshalStreamVersion)

	decoder := yaml.NewDecoder(reader)
	for {
		var node ast.Node
		err := decoder.Decode(&node)
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, fmt.Errorf("error decoding streams: %w", err)
		}

		var docType struct {
			Document string `yaml:"document"`
		}
		if err := yaml.NodeToValue(node, &docType); err != nil {
			return nil, fmt.Errorf("error decoding document type: %w", err)
		}

		if docType.Document == "modulemd" {
			var module ModuleMD
			if err := yaml.NodeToValue(node, &module); err != nil {
				return nil, fmt.Errorf("error decoding modulemd: %w", err)
			}
			moduleMDs = append(moduleMDs, module)
		}
	}
	return moduleMDs, nil
}
