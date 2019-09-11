package unforker

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/ghodss/yaml"
	"github.com/pkg/errors"
	"github.com/replicatedhq/kots/pkg/base"
	yamlv2 "gopkg.in/yaml.v2"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/kustomize/k8sdeps/kunstruct"
	"sigs.k8s.io/kustomize/pkg/resource"
	"sigs.k8s.io/kustomize/v3/pkg/gvk"
	"sigs.k8s.io/kustomize/v3/pkg/types"
)

type MinimalK8sYaml struct {
	ApiVersion string             `json:"apiVersion" yaml:"apiVersion" hcl:"apiVersion"`
	Kind       string             `json:"kind" yaml:"kind"`
	Metadata   MinimalK8sMetadata `json:"metadata" yaml:"metadata"`
}

type MinimalK8sMetadata struct {
	Name      string `json:"name" yaml:"name"`
	Namespace string `json:"namespace" yaml:"namespace"`
}

type namePatch struct {
	PatchJson   string
	PatchTarget types.PatchJson6902
}

func createPatches(forkedPath string, upstreamPath string) (map[string][]byte, map[string][]byte, []namePatch, error) {
	upstreamFiles := map[string][]byte{}
	forkedFiles := map[string][]byte{}
	namePatches := []namePatch{}

	filepath.Walk(forkedPath, func(filename string, info os.FileInfo, err error) error {
		if err != nil {
			panic(err)
		}

		if info.IsDir() {
			return nil
		}

		content, err := ioutil.ReadFile(filename)
		if err != nil {
			return errors.Wrap(err, "failed to read file")
		}

		// ignore files that don't have a gvk-yaml
		o := base.OverlySimpleGVK{}
		if err := yamlv2.Unmarshal(content, &o); err != nil {
			return nil
		}
		if o.APIVersion == "" || o.Kind == "" {
			return nil
		}

		forkedFiles[filename] = content
		return nil
	})

	filepath.Walk(upstreamPath, func(filename string, info os.FileInfo, err error) error {
		if err != nil {
			panic(err)
		}

		if info.IsDir() {
			return nil
		}

		content, err := ioutil.ReadFile(filename)
		if err != nil {
			return errors.Wrap(err, "failed to read file")
		}

		// ignore files that don't have a gvk-yaml
		o := base.OverlySimpleGVK{}
		if err := yamlv2.Unmarshal(content, &o); err != nil {
			return nil
		}
		if o.APIVersion == "" || o.Kind == "" {
			return nil
		}

		upstreamFiles[filename] = content
		return nil
	})

	globalNamePrefix := ""

	// Walk all in the fork, creating patches as needed
	patches := map[string][]byte{}
	resources := map[string][]byte{}
	for filename, content := range forkedFiles {
		upstreamPath, nameprefix, err := findMatchingUpstreamPath(upstreamFiles, content, globalNamePrefix)
		if err != nil {
			return nil, nil, nil, errors.Wrap(err, "failed to find upstream path")
		}

		if upstreamPath == "" {
			_, n := path.Split(filename)
			resources[n] = content
			continue
		}

		if nameprefix != "" {
			globalNamePrefix = nameprefix
			// handle rewriting name of current upstream file AND add a json patch to change that name
			namePatchTarget, namePatchJson, err := createNameReplacePatch(upstreamFiles[upstreamPath], nameprefix)
			if err != nil {
				return nil, nil, nil, errors.Wrap(err, "failed to create name patch for prefix")
			}
			namePatches = append(namePatches, namePatch{PatchJson: namePatchJson, PatchTarget: namePatchTarget})

			// TODO do this with rendering instead of a string replace
			upstreamString := string(upstreamFiles[upstreamPath])
			upstreamString = strings.Replace(upstreamString, fmt.Sprintf("name: %s", namePatchTarget.Target.Name), fmt.Sprintf("name: %s%s", namePatchTarget.Target.Name, nameprefix), 1)
			upstreamFiles[upstreamPath] = []byte(upstreamString)
		}

		patch, err := createTwoWayMergePatch(upstreamFiles[upstreamPath], content)
		if err != nil {
			continue
			// Helm templates. You know?
			// return nil, errors.Wrap(err, "failed to create patch")
		}

		include, err := containsNonGVK(patch)
		if err != nil {
			return nil, nil, nil, errors.Wrap(err, "failed to check if should include patch")
		}

		if include {
			_, n := path.Split(filename)
			patches[n] = patch
		}
	}

	return resources, patches, namePatches, nil
}

// findMatchingUpstreamPath returns the upstream path with a matching gvk+name+ns.
// If no names match, it attempts to find resources whose name has a matching suffix, and also returns the name prefix.
func findMatchingUpstreamPath(upstreamFiles map[string][]byte, forkedContent []byte, suspectedPrefix string) (string, string, error) {
	f := MinimalK8sYaml{}
	if err := yamlv2.Unmarshal(forkedContent, &f); err != nil {
		return "", "", errors.Wrap(err, "failed to unmarshal forked yaml")
	}

	for upstreamFilename, upstreamContents := range upstreamFiles {
		u := MinimalK8sYaml{}
		if err := yamlv2.Unmarshal(upstreamContents, &u); err != nil {
			return "", "", errors.Wrap(err, "failed to unmarshal upstream yaml")
		}

		if u.Kind == f.Kind {
			if u.Metadata.Name == f.Metadata.Name {
				// namespaces match only if they both have one?
				if u.Metadata.Namespace == "" || f.Metadata.Namespace == "" {
					return upstreamFilename, "", nil
				}

				if u.Metadata.Namespace == f.Metadata.Namespace {
					return upstreamFilename, "", nil
				}
			} else if suspectedPrefix+u.Metadata.Name == f.Metadata.Name {
				// if the suspected prefix allows a match, use it preferentially

				// namespaces match only if they both have one?
				if u.Metadata.Namespace == "" || f.Metadata.Namespace == "" {
					return upstreamFilename, suspectedPrefix, nil
				}

				if u.Metadata.Namespace == f.Metadata.Namespace {
					return upstreamFilename, suspectedPrefix, nil
				}
			}
		}
	}

	// if there were no exact name matches, look for name suffix matches
	// 'jaunty-quail-redis' instead of 'redis', for example
	for upstreamFilename, upstreamContents := range upstreamFiles {
		u := MinimalK8sYaml{}
		if err := yamlv2.Unmarshal(upstreamContents, &u); err != nil {
			return "", "", errors.Wrap(err, "failed to unmarshal upstream yaml")
		}

		if u.Kind == f.Kind {
			if strings.HasSuffix(f.Metadata.Name, u.Metadata.Name) {
				prefix := strings.TrimSuffix(f.Metadata.Name, u.Metadata.Name)
				// namespaces match only if they both have one?
				if u.Metadata.Namespace == "" || f.Metadata.Namespace == "" {
					return upstreamFilename, prefix, nil
				}

				if u.Metadata.Namespace == f.Metadata.Namespace {
					return upstreamFilename, prefix, nil
				}
			}
		}
	}

	return "", "", nil
}

func createTwoWayMergePatch(original []byte, modified []byte) ([]byte, error) {
	originalJSON, err := yaml.YAMLToJSON(original)
	if err != nil {
		return nil, errors.Wrap(err, "failed to convert original yaml to json")
	}

	modifiedJSON, err := yaml.YAMLToJSON(modified)
	if err != nil {
		return nil, errors.Wrap(err, "failed to convert modified yaml to json")
	}

	resourceFactory := resource.NewFactory(kunstruct.NewKunstructuredFactoryImpl())
	resources, err := resourceFactory.SliceFromBytes(originalJSON)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse original json")
	}
	if len(resources) != 1 {
		return nil, errors.New("cannot handle > 1 resource")
	}
	originalResource := resources[0]

	versionedObj, err := scheme.Scheme.New(schema.GroupVersionKind{
		Group:   originalResource.Id().Gvk().Group,
		Version: originalResource.Id().Gvk().Version,
		Kind:    originalResource.Id().Gvk().Kind,
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to read gvk from original resource")
	}

	patchBytes, err := strategicpatch.CreateTwoWayMergePatch(originalJSON, modifiedJSON, versionedObj)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create two way merge patch")
	}

	// modifiedPatchJSON, err := p.writeHeaderToPatch(originalJSON, patchBytes)
	// if err != nil {
	// 	return nil, errors.Wrap(err, "write original header to patch")
	// }

	patch, err := yaml.JSONToYAML(patchBytes)
	if err != nil {
		return nil, errors.Wrap(err, "failed to convert patch to yaml")
	}

	return patch, nil
}

func containsNonGVK(data []byte) (bool, error) {
	gvk := []string{
		"apiVersion",
		"kind",
		"metadata",
	}

	unmarshalled := make(map[string]interface{})
	err := yaml.Unmarshal(data, &unmarshalled)
	if err != nil {
		return false, errors.Wrap(err, "failed to unmarshal patch")
	}

	keys := make([]string, 0, 0)
	for k := range unmarshalled {
		keys = append(keys, k)
	}

	for key := range keys {
		isGvk := false
		for gvkKey := range gvk {
			if key == gvkKey {
				isGvk = true
			}
		}

		if !isGvk {
			return true, nil
		}
	}

	return false, nil
}

// createNameReplacePatch creates a json patch that targets the given resource
// and a patch string that adds the supplied prefix to the name.
// the patch filename is not supplied.
func createNameReplacePatch(originalResource []byte, prefix string) (types.PatchJson6902, string, error) {
	unmarshalled := MinimalK8sYaml{}
	err := yaml.Unmarshal(originalResource, &unmarshalled)
	if err != nil {
		return types.PatchJson6902{}, "", errors.Wrap(err, "unmarshal original")
	}

	// just split group and version on '/'
	// TODO better
	gvksplit := strings.Split(unmarshalled.ApiVersion, "/")
	group := gvksplit[0]
	version := ""
	if len(gvksplit) > 1 {
		version = gvksplit[1]
	}

	patch := types.PatchJson6902{
		Target: &types.PatchTarget{
			Gvk: gvk.Gvk{
				Group:   group,
				Version: version,
				Kind:    unmarshalled.Kind,
			},
			Namespace: unmarshalled.Metadata.Namespace,
			Name:      unmarshalled.Metadata.Name,
		},
	}

	// generate the patch json
	patchJsonTemplate := `[{"op":"replace","path":"/metadata/name","value":"%s%s"}]`
	patchJson := fmt.Sprintf(patchJsonTemplate, prefix, unmarshalled.Metadata.Name)

	return patch, patchJson, nil
}
