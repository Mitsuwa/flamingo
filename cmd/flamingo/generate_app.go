package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/flux-subsystem-argo/flamingo/pkg/utils"
	helmv2b1 "github.com/fluxcd/helm-controller/api/v2beta1"
	kustomizev1 "github.com/fluxcd/kustomize-controller/api/v1"
	"github.com/spf13/cobra"
)

var generateAppCmd = &cobra.Command{
	Use:     "generate-app NAME",
	Aliases: []string{"gen-app"},
	Args:    cobra.ExactArgs(1),
	Short:   "Generate a Flamingo application from Flux resources",
	Long: `
# Generate a Flamingo application from a Flux Kustomization podinfo in the current namespace (flux-system).
# The generated application is put in the argocd namespace by default.
flamingo generate-app ks/podinfo

# Generate a Flamingo application from a Flux Kustomization podinfo in the podinfo namespace.
flamingo generate-app -n podinfo ks/podinfo

# Generate a Flamingo application from a HelmRelease podinfo in the current namespace (flux-system).
flamingo generate-app hr/podinfo

# Generate a Flamingo application from a HelmRelease podinfo in the podinfo namespace.
flamingo generate-app -n podinfo hr/podinfo
`,
	RunE: generateAppCmdRun,
}

var generateAppFlags struct {
	appName string
	export  bool
}

func init() {
	generateAppCmd.Flags().StringVar(&generateAppFlags.appName, "app-name", "", "export the generated application to stdout")
	generateAppCmd.Flags().BoolVar(&generateAppFlags.export, "export", false, "export the generated application to stdout")

	rootCmd.AddCommand(generateAppCmd)
}

func generateAppCmdRun(_ *cobra.Command, args []string) error {
	cli, err := utils.KubeClient(kubeconfigArgs, kubeclientOptions)
	if err != nil {
		return err
	}

	kindSlashName := args[0]
	isValid := false
	kindName := ""
	objectName := ""

	// Define a map for valid kinds with their short and full names
	var validKinds = map[string]string{
		"ks":             kustomizev1.KustomizationKind,
		"kustomization":  kustomizev1.KustomizationKind,
		"kustomizations": kustomizev1.KustomizationKind,
		"hr":             helmv2b1.HelmReleaseKind,
		"helmrelease":    helmv2b1.HelmReleaseKind,
		"helmreleases":   helmv2b1.HelmReleaseKind,
	}
	// Check if the kindSlashName starts with any of the valid prefixes
	for shortName, fullName := range validKinds {
		if strings.HasPrefix(kindSlashName, shortName+"/") {
			isValid = true
			kindName = fullName
			break
		} else if strings.HasPrefix(kindSlashName, fullName+"/") {
			isValid = true
			kindName = fullName
			break
		}
	}

	if !isValid {
		return fmt.Errorf("not a valid Kustomization or HelmRelease resource")
	}

	objectName = strings.Split(kindSlashName, "/")[1]
	appName := generateAppFlags.appName
	if appName == "" {
		appName = objectName
	}

	var tpl bytes.Buffer
	if kindName == kustomizev1.KustomizationKind {
		if err := generateKustomizationApp(cli, appName, objectName, kindName, &tpl); err != nil {
			return err
		}
	} else if kindName == helmv2b1.HelmReleaseKind {
		if err := generateHelmReleaseApp(cli, appName, objectName, kindName, &tpl); err != nil {
			return err
		}
	}

	if generateAppFlags.export {
		fmt.Print(tpl.String())
		return nil
	} else {
		logger.Actionf("applying generated application %s in %s namespace", objectName, rootArgs.applicationNamespace)
		applyOutput, err := utils.Apply(context.Background(), kubeconfigArgs, kubeclientOptions, tpl.Bytes())
		if err != nil {
			return fmt.Errorf("install failed: %w", err)
		}
		fmt.Fprintln(os.Stderr, applyOutput)
	}

	return nil
}
