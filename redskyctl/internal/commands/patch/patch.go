/*
Copyright 2020 GramLabs, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package patch

import (
	"context"
	"fmt"

	"github.com/redskyops/redskyops-controller/internal/config"
	"github.com/redskyops/redskyops-controller/redskyctl/internal/commander"
	"github.com/redskyops/redskyops-controller/redskyctl/internal/kustomize"
	"github.com/spf13/cobra"
	"sigs.k8s.io/kustomize/api/types"
)

// GeneratorOptions are the configuration options for creating a patched experiment
type PatchOptions struct {
	// Config is the Red Sky Configuration used to generate the controller installation
	Config *config.RedSkyConfig
	// IOStreams are used to access the standard process streams
	commander.IOStreams
}

// Options is the configuration for initialization
type Options struct {
	PatchOptions

	experimentFilename string
	manifestsFilename  string
}

// NewCommand creates a command for performing an initialization
func NewCommand(o *Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "patch",
		Short: "patchy patch",
		Long:  "patchy patchy patch",

		PreRun: commander.StreamsPreRun(&o.IOStreams),
		RunE:   commander.WithContextE(o.patch),
	}

	cmd.Flags().StringVar(&o.experimentFilename, "e", "", "experiment filename")
	cmd.Flags().StringVar(&o.manifestsFilename, "m", "", "manifests filename")

	return cmd
}

func (o *Options) patch(ctx context.Context) error {
	// s/[]byte{}/-f flag

	// readExperiments(o.experimentFilename, o.In)
	assets := kustomize.NewAssetFromBytes([]byte{})
	// render patches
	patches := map[string]types.Patch{}

	yamls, err := kustomize.Yamls(
		kustomize.WithResources(map[string]*kustomize.Asset{"experiment": assets}),
		kustomize.WithPatches(patches),
	)
	if err != nil {
		return err
	}

	fmt.Fprintln(o.Out, string(yamls))

	return nil
}
