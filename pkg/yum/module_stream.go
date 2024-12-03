package yum

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/mitchellh/mapstructure"
	"gopkg.in/yaml.v3"
)

// Better userfacing struct
type ModuleStream struct {
	Name    string
	Streams []Stream
}

type Stream struct {
	Name        string                 `mapstructure:"name"`
	Stream      string                 `mapstructure:"stream"`
	Version     string                 `mapstructure:"version"`
	Context     string                 `mapstructure:"context"`
	Arch        string                 `mapstructure:"arch"`
	Summary     string                 `mapstructure:"summary"`
	Description string                 `mapstructure:"description"`
	Artifacts   Artifacts              `mapstructure:"artifacts"`
	Profiles    map[string]RpmProfiles `mapstructure:"profiles"`
}

type RpmProfiles struct {
	Rpms []string `mapstructure:"rpms"`
}

type Artifacts struct {
	Rpms []string `mapstructure:"rpms"`
}

type ModuleMD struct {
	Document string `mapstructure:"document"`
	Version  int    `mapstructure:"version"`
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

		if resp, err = r.settings.Client.Do(req); err != nil {
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

// parses modulemd objects from a given io reader
// modules yaml files include different types of documents which is hard to parse
// this implements a two step process:
//
//	Parse each document into a map, with the value of interface, and then
//	use mapstructure to parse the interface into a ModuleMD struct
func parseModuleMDs(body io.ReadCloser) ([]ModuleMD, error) {
	moduleMDs := make([]ModuleMD, 0)

	reader, err := ExtractIfCompressed(body)
	if err != nil {
		return moduleMDs, fmt.Errorf("error extracting compressed streams: %w", err)
	}

	decoder := yaml.NewDecoder(reader)
	for {
		var doc map[string]interface{}

		// Decode the next document
		err := decoder.Decode(&doc)
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, fmt.Errorf("error decoding streams: %w", err)
		}
		// Only care about modulemds right now
		if doc["document"] == "modulemd" {
			var module ModuleMD
			config := &mapstructure.DecoderConfig{
				WeaklyTypedInput: true,
				Result:           &module,
			}
			mapDecode, err := mapstructure.NewDecoder(config)
			if err != nil {
				return moduleMDs, fmt.Errorf("error creating map decoder: %w", err)
			}
			err = mapDecode.Decode(doc)
			if err != nil {
				return nil, fmt.Errorf("error decoding map: %w", err)
			}
			moduleMDs = append(moduleMDs, module)
		}
	}
	return moduleMDs, nil
}
