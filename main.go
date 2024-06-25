package main

import (
	"log"
	"os"
)

func main() {
	// read the command line arguments for openapi file and data folder
	if len(os.Args) != 3 {
		log.Fatalf("Usage: %s <openapi-file> <target-folder>", os.Args[0])
	}
	openApiFile := os.Args[1]
	targetFolder := os.Args[2]

	// validate the openapi file existence
	if _, err := os.Stat(openApiFile); os.IsNotExist(err) {
		log.Fatalf("OpenAPI file does not exist: %s", openApiFile)
	}

	// verify if traget folder does not exist
	if _, err := os.Stat(targetFolder); os.IsNotExist(err) {
		log.Fatalf("Failed to create data folder: %v", err)
	}

	log.Printf("Exporting OpenAPI to mock server: %s -> %s\n", openApiFile, targetFolder)

	// export OpenAPI to mock server
	exportOpenAPIToMockServer(openApiFile, targetFolder)
}

func exportOpenAPIToMockServer(openApiFile string, targetFolder string) {
	// Step 1: Read the OpenAPI file.
	openAPISpec := ParseOpenApiFile(openApiFile)

	// Step 2: Convert OpenAPI to mock server.
	mockServerInfo := ConvertOpenAPIToMockServer(openAPISpec)

	// Step 3: Create mock server data folder.
	mockServerInfo.CreateFolder(targetFolder)

	// Step 4: Output mock server setting file
	mockServerInfo.SaveSetting()

	// step 5: copy the openapi file to the data folder
	mockServerInfo.CopyOpenAPIFile(openApiFile)
}
