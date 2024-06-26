package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
	"gopkg.in/yaml.v3"
)

// MockServerSetting defines the structure of mock server.

type MockServerSetting struct {
	Name           string            `yaml:"name"`
	Description    string            `yaml:"description"`
	Folder         string            `yaml:"-"` // Folder is not saved in the YAML file
	Host           string            `yaml:"host"`
	Port           int               `yaml:"port"`
	SwaggerEnabled bool              `yaml:"swaggerEnabled"`
	Headers        *[]Header         `yaml:"headers,omitempty"`
	Requests       []Request         `yaml:"requests"`
	Schemas        map[string]string `yaml:"-"`
}

type Request struct {
	Name      string     `yaml:"name"`
	Method    string     `yaml:"method"`
	Path      string     `yaml:"path"`
	Responses []Response `yaml:"responses"`
}

type Response struct {
	Name     string    `yaml:"name"`
	Code     int       `yaml:"code"`
	Query    string    `yaml:"query,omitempty"`
	Headers  *[]Header `yaml:"headers,omitempty"`
	FilePath *string   `yaml:"filePath,omitempty"`
	Body     *string   `yaml:"-"` // Body is not saved in the YAML file
}

type Header struct {
	Name  string `yaml:"name"`
	Value string `yaml:"value"`
}

func ParseOpenApiFile(openApiFile string) openapi3.T {
	data, err := os.ReadFile(openApiFile)
	if err != nil {
		log.Fatalf("Failed to read OpenAPI file: %v", err)
	}

	// Step 2: Parse the OpenAPI file.
	loader := openapi3.NewLoader()
	openAPISpec, err := loader.LoadFromData(data)
	if err != nil || openAPISpec == nil {
		log.Fatalf("Failed to parse OpenAPI file: %v", err)
	}

	return *openAPISpec
}

// ConvertOpenAPIToCustomFormat converts an OpenAPI spec to mock server.
func ConvertOpenAPIToMockServer(openAPISpec openapi3.T) MockServerSetting {
	headers := getHeaders(openAPISpec)
	requests := getRequests(openAPISpec)
	return MockServerSetting{
		Name:           openAPISpec.Info.Title,
		Description:    openAPISpec.Info.Description,
		Host:           "0.0.0.0",
		Port:           randomPort(),
		SwaggerEnabled: true,
		Headers:        &headers,
		Requests:       requests,
	}
}

// RandomPort generates a random port number from 10000 to 60000.
func randomPort() int {
	return 10000 + (os.Getpid() % 50000)
}

func getHeaders(openAPISpec openapi3.T) []Header {
	return []Header{}
}

// getRequests extracts the requests from the OpenAPI spec.
func getRequests(openAPISpec openapi3.T) (requests []Request) {
	// Loop through the components
	schemaExamples := make(map[string]string)
	if openAPISpec.Components != nil && openAPISpec.Components.Schemas != nil {
		for schemaName, schemaRef := range openAPISpec.Components.Schemas {
			schema := schemaRef.Value
			// Extract the schema
			schemaFullName := fmt.Sprintf("#/components/schemas/%s", schemaName)
			schemaExample := extractSchemaExample(schema)
			schemaExamples[schemaFullName] = schemaExample
		}
	}
	// Loop through the paths
	for path, pathItem := range openAPISpec.Paths.Map() {
		for method, operation := range pathItem.Operations() {
			fmt.Printf("Path: %s, Method: %s, Operation: %s\n", path, method, operation.OperationID)

			// Extract the responses
			responses := extractResponse(operation, schemaExamples)

			// Sort responses by code
			sort.Slice(responses, func(i, j int) bool {
				return responses[i].Code < responses[j].Code
			})

			// Create a request object
			requests = append(requests, Request{
				Name:      operation.OperationID,
				Method:    method,
				Path:      path,
				Responses: responses,
			})
		}
	}
	return requests
}

func extractResponse(operation *openapi3.Operation, schemaExamples map[string]string) []Response {
	responses := []Response{}

	// Loop through the responses
	for response, responseItem := range operation.Responses.Map() {
		// Get the description of the response
		var description = ""
		if responseItem.Value.Description != nil {
			description = *responseItem.Value.Description
		}

		// Get the response code
		code, err := strconv.Atoi(response)
		if err != nil {
			log.Fatalf("Failed to convert response code to integer: %v", err)
		}

		// Get the content type
		contentType := ""
		if responseItem.Value != nil {
			if responseItem.Value.Content != nil {
				for contentType = range responseItem.Value.Content {
					headers := []Header{
						{Name: "Content-Type", Value: contentType},
					}
					content := responseItem.Value.Content[contentType]
					if content == nil {
						continue
					}
					examples := content.Examples
					schema := content.Schema
					if len(examples) > 0 {
						for exampleName, examapleObject := range examples {
							bodyStr := getBodyString(examapleObject)

							// Create a response object
							response := Response{
								Name:    cleanFolderName(description),
								Code:    code,
								Query:   "?key=" + response + "&contentType=" + contentType + "&name=" + exampleName,
								Headers: &headers,
							}
							if len(bodyStr) > 0 {
								response.Body = &bodyStr
							}
							responses = append(responses, response)
						}
					} else if schema != nil && schema.Ref != "" {
						bodyStr, ok := schemaExamples[schema.Ref]
						if ok && bodyStr != "" {
							responses = append(responses, Response{
								Name:    cleanFolderName(description),
								Code:    code,
								Query:   "?key=" + response + "&contentType=" + contentType,
								Headers: &headers,
								Body:    &bodyStr,
							})
						} else {
							responses = append(responses, Response{
								Name:    cleanFolderName(description),
								Code:    code,
								Query:   "?key=" + response + "&contentType=" + contentType,
								Headers: &headers,
							})
						}
					} else {
						responses = append(responses, Response{
							Name:    cleanFolderName(description),
							Code:    code,
							Query:   "?key=" + response + "&contentType=" + contentType,
							Headers: &headers,
						})
					}
				}
			} else {
				responses = append(responses, Response{
					Name:  cleanFolderName(description),
					Code:  code,
					Query: "?key=" + strconv.Itoa(code),
				})
			}
		}
	}
	return responses
}

func getBodyString(exampleRef *openapi3.ExampleRef) string {
	if exampleRef == nil || exampleRef.Value == nil {
		return ""
	}
	examapleObject := exampleRef.Value.Value
	if examapleObject == nil {
		return ""
	}
	// there are 2 types of body that we can support, string and object
	// for string, just use it as is
	// for object, convert it to string

	bodyStr := ""
	if _, ok := examapleObject.(string); ok {
		bodyStr = fmt.Sprintf("%s", examapleObject)
	} else {
		jsonData, err := json.MarshalIndent(examapleObject, "", "  ")
		if err != nil {
			return ""
		}
		bodyStr = string(jsonData)
	}
	return bodyStr
}

func extractSchemaExample(schema *openapi3.Schema) string {
	om := NewOrderedMap()

	schemaType := schema.Type
	if schemaType.Is("object") {
		// Extract the properties
		if schema.Properties != nil {
			for propName, propSchema := range schema.Properties {
				childSchema := propSchema.Value
				childSchemaType := childSchema.Type
				if childSchemaType.Is("string") || childSchemaType.Is("integer") {
					om.Set(propName, childSchema.Example)
				}
			}
		}
	}

	// Marshal the schema to JSON
	finalData, err := json.MarshalIndent(om, "", "  ")
	if err != nil {
		return ""
	}
	return string(finalData)
}

// cleanFolderName takes a string and returns a valid folder name by first trimming
// leading and trailing spaces, replacing internal spaces with underscores, and
// removing characters that are not allowed in folder names.
func cleanFolderName(name string) string {
	// Replace new line with empty string
	name = strings.ReplaceAll(name, "\n", "")

	// Trim leading and trailing spaces
	name = strings.TrimSpace(name)

	// Define a list of characters that are not allowed in file names
	notAllowedChars := []string{"<", ">", ":", "\"", "/", "\\", "|", "?", "*"}

	// Replace spaces with underscores
	cleanName := strings.ReplaceAll(name, " ", "_")

	// Remove not allowed characters
	for _, char := range notAllowedChars {
		cleanName = strings.ReplaceAll(cleanName, char, "")
	}

	return cleanName
}

func (m *MockServerSetting) CreateFolder(targetFolder string) {
	// Clean the folder name
	folderName := cleanFolderName(m.Name)

	// Trim right slash
	targetFolder = strings.TrimRight(targetFolder, "/")
	targetFolder = strings.TrimRight(targetFolder, "\\")

	// Set the folder path for the mock server
	m.Folder = fmt.Sprintf("%s/data/%s", targetFolder, folderName)

	// Create the data folder if it does not exist
	if _, err := os.Stat(m.Folder); os.IsNotExist(err) {
		if err := os.Mkdir(m.Folder, 0755); err != nil {
			log.Fatalf("Failed to create data folder: %v", err)
		}
	}
}

// SaveSetting saves the mock server setting to a file.
// Save response files for each request
func (m *MockServerSetting) SaveSetting() {
	// Create folder for each response
	for i, request := range m.Requests {
		for j, response := range request.Responses {
			folderRelativePath := fmt.Sprintf("%s/%s/%d", request.Method, request.Name, response.Code)
			folderFullPath := fmt.Sprintf("%s/%s", m.Folder, folderRelativePath)
			fileName := cleanFolderName(response.Name)
			fileRelativePath := fmt.Sprintf("./data/%s/%s/%s.json", cleanFolderName(m.Name), folderRelativePath, fileName)
			fileFullPath := fmt.Sprintf("%s/%s.json", folderFullPath, fileName)

			if response.Body != nil {
				// Save the folder path to the response
				response.FilePath = &fileRelativePath
				m.Requests[i].Responses[j] = response

				// Create a folder for the response
				if _, err := os.Stat(folderFullPath); os.IsNotExist(err) {
					if err := os.MkdirAll(folderFullPath, 0755); err != nil {
						log.Fatalf("Failed to create response folder: %v", err)
					}
				}

				// Save the response body to a file
				if err := os.WriteFile(fileFullPath, []byte(*response.Body), 0644); err != nil {
					log.Fatalf("Failed to write response body to file: %v", err)
				}
				log.Printf("Response body is saved to %s\n", *response.FilePath)
			}
		}
	}

	// Create the setting file
	settingFilePath := fmt.Sprintf("%s/setting.yaml", m.Folder)
	file, err := os.Create(settingFilePath)
	if err != nil {
		log.Fatalf("Failed to create mock server setting file: %v", err)
	}
	defer file.Close()

	// Marshal the mock server setting to YAML format
	encoder := yaml.NewEncoder(file)
	encoder.SetIndent(2) // Indent by 2 spaces
	if encoder.Encode(m) != nil {
		log.Fatalf("Failed to write mock server setting to file: %v", err)
	}

	fmt.Printf("Mock server setting is saved to %s\n", settingFilePath)
}

func (m *MockServerSetting) CopyOpenAPIFile(openApiFile string) {
	data, _ := os.ReadFile(openApiFile)
	filePath := m.Folder + "/openapi" + filepath.Ext(openApiFile)
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		log.Fatalf("Failed to copy OpenAPI file to data folder: %v", err)
	} else {
		log.Printf("OpenAPI file copied to data folder: %s", filePath)
	}
}
