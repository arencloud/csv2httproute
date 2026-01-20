package main

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// HTTPRoute structs based on the CRD
type HTTPRoute struct {
	APIVersion string        `yaml:"apiVersion"`
	Kind       string        `yaml:"kind"`
	Metadata   Metadata      `yaml:"metadata"`
	Spec       HTTPRouteSpec `yaml:"spec"`
}

type Metadata struct {
	Name      string            `yaml:"name"`
	Namespace string            `yaml:"namespace,omitempty"`
	Labels    map[string]string `yaml:"labels,omitempty"`
}

type HTTPRouteSpec struct {
	ParentRefs []ParentRef     `yaml:"parentRefs,omitempty"`
	Hostnames  []string        `yaml:"hostnames,omitempty"`
	Rules      []HTTPRouteRule `yaml:"rules,omitempty"`
}

type ParentRef struct {
	Group     string `yaml:"group,omitempty"`
	Kind      string `yaml:"kind,omitempty"`
	Name      string `yaml:"name"`
	Namespace string `yaml:"namespace,omitempty"`
}

type HTTPRouteRule struct {
	Matches     []HTTPRouteMatch  `yaml:"matches,omitempty"`
	Filters     []HTTPRouteFilter `yaml:"filters,omitempty"`
	BackendRefs []BackendRef      `yaml:"backendRefs,omitempty"`
}

type HTTPRouteMatch struct {
	Path   *HTTPPathMatch `yaml:"path,omitempty"`
	Method string         `yaml:"method,omitempty"`
}

type HTTPPathMatch struct {
	Type  string `yaml:"type,omitempty"`
	Value string `yaml:"value,omitempty"`
}

type HTTPRouteFilter struct {
	Type       string            `yaml:"type"`
	URLRewrite *URLRewriteFilter `yaml:"urlRewrite,omitempty"`
}

type URLRewriteFilter struct {
	Path *PathRewrite `yaml:"path,omitempty"`
}

type PathRewrite struct {
	Type               string `yaml:"type,omitempty"`
	ReplacePrefixMatch string `yaml:"replacePrefixMatch,omitempty"`
}

type BackendRef struct {
	Group     string `yaml:"group,omitempty"`
	Kind      string `yaml:"kind,omitempty"`
	Name      string `yaml:"name"`
	Namespace string `yaml:"namespace,omitempty"`
	Port      int    `yaml:"port,omitempty"`
	Weight    int    `yaml:"weight,omitempty"`
}

type Endpoint struct {
	Method  string
	URL     string
	Prefix  string
	Comment string
}

var (
	Version = "v1.0.0"
)

var (
	inputDir         string
	outputDir        string
	serviceName      string
	servicePort      int
	serviceNamespace string
	gatewayName      string
	gatewayNamespace string
	namespace        string
	hostname         string
)

func main() {
	var rootCmd = &cobra.Command{
		Use:     "csv2httproute",
		Short:   "Generate K8s HTTPRoute from CSV endpoints",
		Version: Version,
		RunE:    run,
	}

	rootCmd.Flags().StringVarP(&inputDir, "input", "i", "facts/endpoints", "Directory or CSV file to process")
	rootCmd.Flags().StringVarP(&outputDir, "output", "o", "generated", "Output directory for YAML files")
	rootCmd.Flags().StringVarP(&serviceName, "service", "s", "my-service", "Default backend service name")
	rootCmd.Flags().IntVarP(&servicePort, "port", "p", 80, "Default backend service port")
	rootCmd.Flags().StringVar(&serviceNamespace, "service-namespace", "", "Namespace for the backend service")
	rootCmd.Flags().StringVarP(&gatewayName, "gateway", "g", "my-gateway", "Parent gateway name")
	rootCmd.Flags().StringVar(&gatewayNamespace, "gateway-namespace", "", "Namespace for the parent gateway (defaults to --namespace)")
	rootCmd.Flags().StringVarP(&namespace, "namespace", "n", "default", "Namespace for HTTPRoute")
	rootCmd.Flags().StringVar(&hostname, "hostname", "", "Hostname for the HTTPRoute")

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(cmd *cobra.Command, args []string) error {
	if _, err := os.Stat(outputDir); os.IsNotExist(err) {
		if err := os.MkdirAll(outputDir, 0755); err != nil {
			return fmt.Errorf("failed to create output directory: %w", err)
		}
	}

	info, err := os.Stat(inputDir)
	if err != nil {
		return fmt.Errorf("failed to access input: %w", err)
	}

	if !info.IsDir() {
		if !strings.HasSuffix(inputDir, ".csv") {
			return fmt.Errorf("input file must be a CSV file")
		}
		return processCSV(inputDir)
	}

	files, err := os.ReadDir(inputDir)
	if err != nil {
		return fmt.Errorf("failed to read input directory: %w", err)
	}

	for _, file := range files {
		if !file.IsDir() && strings.HasSuffix(file.Name(), ".csv") {
			if err := processCSV(filepath.Join(inputDir, file.Name())); err != nil {
				fmt.Printf("Error processing %s: %v\n", file.Name(), err)
			}
		}
	}

	return nil
}

func processCSV(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	reader := csv.NewReader(f)
	reader.FieldsPerRecord = -1 // Allow variable number of fields
	// Read header
	header, err := reader.Read()
	if err != nil {
		return err
	}

	headerMap := make(map[string]int)
	for i, h := range header {
		headerMap[strings.ToLower(strings.TrimSpace(h))] = i
	}

	var endpoints []Endpoint
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		if len(record) == 0 || (len(record) > 0 && strings.HasPrefix(strings.TrimSpace(record[0]), "#")) {
			continue
		}

		endpoint := parseRecord(record, headerMap)
		if endpoint.URL == "" {
			continue
		}
		endpoints = append(endpoints, endpoint)
	}

	if len(endpoints) == 0 {
		return nil
	}

	baseName := strings.TrimSuffix(filepath.Base(path), ".csv")
	// Clean up name for K8s resource
	resourceName := strings.ReplaceAll(baseName, "endpoints-", "")
	resourceName = strings.ReplaceAll(resourceName, "_", "-")

	effectiveGatewayNamespace := gatewayNamespace
	if effectiveGatewayNamespace == "" {
		effectiveGatewayNamespace = namespace
	}

	route := HTTPRoute{
		APIVersion: "gateway.networking.k8s.io/v1",
		Kind:       "HTTPRoute",
		Metadata: Metadata{
			Name:      resourceName,
			Namespace: namespace,
		},
		Spec: HTTPRouteSpec{
			ParentRefs: []ParentRef{
				{
					Group:     "gateway.networking.k8s.io",
					Kind:      "Gateway",
					Name:      gatewayName,
					Namespace: effectiveGatewayNamespace,
				},
			},
		},
	}

	if hostname != "" {
		route.Spec.Hostnames = []string{hostname}
	}

	// Group endpoints by prefix
	prefixGroups := make(map[string][]Endpoint)
	var prefixes []string // To maintain order if needed, but map is fine for now

	for _, e := range endpoints {
		if e.Prefix != "" {
			if _, ok := prefixGroups[e.Prefix]; !ok {
				prefixes = append(prefixes, e.Prefix)
			}
			prefixGroups[e.Prefix] = append(prefixGroups[e.Prefix], e)
		}
	}

	// Create rules for prefixes
	for _, prefix := range prefixes {
		// Rule 1: Match Prefix and Rewrite to /
		rule1 := HTTPRouteRule{
			Matches: []HTTPRouteMatch{
				{
					Path: &HTTPPathMatch{
						Type:  "PathPrefix",
						Value: prefix,
					},
				},
			},
			Filters: []HTTPRouteFilter{
				{
					Type: "URLRewrite",
					URLRewrite: &URLRewriteFilter{
						Path: &PathRewrite{
							Type:               "ReplacePrefixMatch",
							ReplacePrefixMatch: "/",
						},
					},
				},
			},
			BackendRefs: []BackendRef{
				{
					Group:     "",
					Kind:      "Service",
					Name:      serviceName,
					Namespace: serviceNamespace,
					Port:      servicePort,
					Weight:    1,
				},
			},
		}
		route.Spec.Rules = append(route.Spec.Rules, rule1)
	}

	// Rule 2: Direct matches for all URLs (from all prefixes and no-prefix)
	rule2 := HTTPRouteRule{
		BackendRefs: []BackendRef{
			{
				Group:     "",
				Kind:      "Service",
				Name:      serviceName,
				Namespace: serviceNamespace,
				Port:      servicePort,
				Weight:    1,
			},
		},
	}
	for _, e := range endpoints {
		rule2.Matches = append(rule2.Matches, HTTPRouteMatch{
			Path: &HTTPPathMatch{
				Type:  "PathPrefix",
				Value: e.URL,
			},
			Method: strings.ToUpper(e.Method),
		})
	}
	route.Spec.Rules = append(route.Spec.Rules, rule2)

	outPath := filepath.Join(outputDir, resourceName+".yaml")
	outFile, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer outFile.Close()

	encoder := yaml.NewEncoder(outFile)
	encoder.SetIndent(2)
	if err := encoder.Encode(route); err != nil {
		return err
	}

	fmt.Printf("Generated %s\n", outPath)
	return nil
}

func parseRecord(record []string, headerMap map[string]int) Endpoint {
	e := Endpoint{}
	if idx, ok := headerMap["method"]; ok && idx < len(record) {
		e.Method = strings.TrimSpace(record[idx])
	}
	if idx, ok := headerMap["url"]; ok && idx < len(record) {
		e.URL = strings.TrimSpace(record[idx])
	}
	if idx, ok := headerMap["prefix"]; ok && idx < len(record) {
		e.Prefix = strings.TrimSpace(record[idx])
	}
	if idx, ok := headerMap["comment"]; ok && idx < len(record) {
		e.Comment = strings.TrimSpace(record[idx])
	}
	return e
}
