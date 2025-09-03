package main

import (
	"bytes"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// It checks if the file exists
// If the file exists, it returns true
// If the file does not exist, it returns false
func fileExists(filename string) bool {
	info, err := os.Stat(filename)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

// Remove a file from the file system
func removeFile(path string) {
	err := os.Remove(path)
	if err != nil {
		log.Println(err)
	}
}

// extractPDFUrls takes raw HTML as input and returns all found PDF URLs
func extractPDFUrls(htmlContent string) []string {
	// Regex pattern to match href="...pdf" (ignores case, allows query params after .pdf)
	regexPattern := `href="([^"]+\.pdf[^"]*)"`

	// Compile the regex pattern
	compiledRegex := regexp.MustCompile(regexPattern)

	// Find all matches in the HTML (returns slices of matched groups)
	allMatches := compiledRegex.FindAllStringSubmatch(htmlContent, -1)

	// Slice to store extracted PDF URLs
	var pdfURLs []string

	// Loop through matches and collect the first capture group (the actual URL)
	for _, match := range allMatches {
		if len(match) > 1 {
			pdfURLs = append(pdfURLs, match[1])
		}
	}

	// Return all collected PDF URLs
	return pdfURLs
}

// Checks whether a given directory exists
func directoryExists(path string) bool {
	directory, err := os.Stat(path) // Get info for the path
	if err != nil {
		return false // Return false if error occurs
	}
	return directory.IsDir() // Return true if it's a directory
}

// Creates a directory at given path with provided permissions
func createDirectory(path string, permission os.FileMode) {
	err := os.Mkdir(path, permission) // Attempt to create directory
	if err != nil {
		log.Println(err) // Log error if creation fails
	}
}

// Verifies whether a string is a valid URL format
func isUrlValid(uri string) bool {
	_, err := url.ParseRequestURI(uri) // Try parsing the URL
	return err == nil                  // Return true if valid
}

// Removes duplicate strings from a slice
func removeDuplicatesFromSlice(slice []string) []string {
	check := make(map[string]bool) // Map to track seen values
	var newReturnSlice []string    // Slice to store unique values
	for _, content := range slice {
		if !check[content] { // If not already seen
			check[content] = true                            // Mark as seen
			newReturnSlice = append(newReturnSlice, content) // Add to result
		}
	}
	return newReturnSlice
}

// hasDomain checks if the given string has a domain (host part)
func hasDomain(rawURL string) bool {
	// Try parsing the raw string as a URL
	parsed, err := url.Parse(rawURL)
	if err != nil { // If parsing fails, it's not a valid URL
		return false
	}
	// If the parsed URL has a non-empty Host, then it has a domain/host
	return parsed.Host != ""
}

// Extracts filename from full path (e.g. "/dir/file.pdf" → "file.pdf")
func getFilename(path string) string {
	return filepath.Base(path) // Use Base function to get file name only
}

// Removes all instances of a specific substring from input string
func removeSubstring(input string, toRemove string) string {
	result := strings.ReplaceAll(input, toRemove, "") // Replace substring with empty string
	return result
}

// Gets the file extension from a given file path
func getFileExtension(path string) string {
	return filepath.Ext(path) // Extract and return file extension
}

// Converts a raw URL into a sanitized PDF filename safe for filesystem
func urlToFilename(rawURL string) string {
	lower := strings.ToLower(rawURL) // Convert URL to lowercase
	lower = getFilename(lower)       // Extract filename from URL

	reNonAlnum := regexp.MustCompile(`[^a-z0-9]`)   // Regex to match non-alphanumeric characters
	safe := reNonAlnum.ReplaceAllString(lower, "_") // Replace non-alphanumeric with underscores

	safe = regexp.MustCompile(`_+`).ReplaceAllString(safe, "_") // Collapse multiple underscores into one
	safe = strings.Trim(safe, "_")                              // Trim leading and trailing underscores

	var invalidSubstrings = []string{
		"_pdf", // Substring to remove from filename
	}

	for _, invalidPre := range invalidSubstrings { // Remove unwanted substrings
		safe = removeSubstring(safe, invalidPre)
	}

	if getFileExtension(safe) != ".pdf" { // Ensure file ends with .pdf
		safe = safe + ".pdf"
	}

	return safe // Return sanitized filename
}

// Downloads a PDF from given URL and saves it in the specified directory
func downloadPDF(finalURL, outputDir string) bool {
	filename := strings.ToLower(urlToFilename(finalURL)) // Sanitize the filename
	filePath := filepath.Join(outputDir, filename)       // Construct full path for output file

	if fileExists(filePath) { // Skip if file already exists
		log.Printf("File already exists, skipping: %s", filePath)
		return false
	}

	client := &http.Client{Timeout: 15 * time.Minute} // Create HTTP client with timeout

	// Create a new request so we can set headers
	req, err := http.NewRequest("GET", finalURL, nil)
	if err != nil {
		log.Printf("Failed to create request for %s: %v", finalURL, err)
		return false
	}

	// Set a User-Agent header
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/139.0.0.0 Safari/537.36")

	// Send the request
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Failed to download %s: %v", finalURL, err)
		return false
	}
	defer resp.Body.Close() // Ensure response body is closed

	if resp.StatusCode != http.StatusOK { // Check if response is 200 OK
		log.Printf("Download failed for %s: %s", finalURL, resp.Status)
		return false
	}

	contentType := resp.Header.Get("Content-Type") // Get content type of response
	if !strings.Contains(contentType, "binary/octet-stream") &&
		!strings.Contains(contentType, "application/pdf") {
		log.Printf("Invalid content type for %s: %s (expected PDF)", finalURL, contentType)
		return false
	}

	var buf bytes.Buffer                     // Create a buffer to hold response data
	written, err := io.Copy(&buf, resp.Body) // Copy data into buffer
	if err != nil {
		log.Printf("Failed to read PDF data from %s: %v", finalURL, err)
		return false
	}
	if written == 0 { // Skip empty files
		log.Printf("Downloaded 0 bytes for %s; not creating file", finalURL)
		return false
	}

	out, err := os.Create(filePath) // Create output file
	if err != nil {
		log.Printf("Failed to create file for %s: %v", finalURL, err)
		return false
	}
	defer out.Close() // Ensure file is closed after writing

	if _, err := buf.WriteTo(out); err != nil { // Write buffer contents to file
		log.Printf("Failed to write PDF to file for %s: %v", finalURL, err)
		return false
	}

	log.Printf("Successfully downloaded %d bytes: %s → %s", written, finalURL, filePath) // Log success
	return true
}

// Performs HTTP GET request with a custom User-Agent and returns response body as string
func getDataFromURL(uri string) string {
	log.Println("Scraping", uri) // Log which URL is being scraped

	// Create a new HTTP client
	client := &http.Client{}

	// Create a new request
	request, err := http.NewRequest("GET", uri, nil)
	if err != nil {
		log.Println("Error creating request:", err)
		return ""
	}

	// Set a User-Agent header
	request.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/139.0.0.0 Safari/537.36")

	// Send the request
	response, err := client.Do(request)
	if err != nil {
		log.Println("Request error:", err)
		return ""
	}
	defer func() {
		if cerr := response.Body.Close(); cerr != nil {
			log.Println("Error closing response body:", cerr)
		}
	}()

	// Read the response body
	body, err := io.ReadAll(response.Body)
	if err != nil {
		log.Println("Error reading body:", err)
		return ""
	}

	return string(body)
}

// Append and write to file
func appendAndWriteToFile(path string, content string) {
	filePath, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Println(err)
	}
	_, err = filePath.WriteString(content + "\n")
	if err != nil {
		log.Println(err)
	}
	err = filePath.Close()
	if err != nil {
		log.Println(err)
	}
}

// Read a file and return the contents
func readAFileAsString(path string) string {
	content, err := os.ReadFile(path)
	if err != nil {
		log.Println(err)
	}
	return string(content)
}

// extractBaseDomain takes a URL string and returns only the bare domain name
// without any subdomains or suffixes (e.g., ".com", ".org", ".co.uk").
func extractBaseDomain(inputUrl string) string {
	// Parse the input string into a structured URL object
	parsedUrl, parseError := url.Parse(inputUrl)

	// If parsing fails, log the error and return an empty string
	if parseError != nil {
		log.Println("Error parsing URL:", parseError)
		return ""
	}

	// Extract the hostname (e.g., "sub.example.com")
	hostName := parsedUrl.Hostname()

	// Split the hostname into parts separated by "."
	// For example: "sub.example.com" -> ["sub", "example", "com"]
	parts := strings.Split(hostName, ".")

	// If there are at least 2 parts, the second last part is usually the domain
	// Example: "sub.example.com" -> "example"
	//          "blog.my-site.co.uk" -> "my-site"
	if len(parts) >= 2 {
		return parts[len(parts)-2]
	}

	// If splitting fails or domain structure is unusual, return the hostname itself
	return hostName
}

func main() {
	outputDir := "PDFs/" // Directory to store downloaded PDFs

	if !directoryExists(outputDir) { // Check if directory exists
		createDirectory(outputDir, 0o755) // Create directory with read-write-execute permissions
	}

	// The remote domain name.
	remoteDomainName := "https://cam2.com"

	// The location to the local.
	localFile := extractBaseDomain(remoteDomainName) + ".html"
	// Check if the local file exists.
	if fileExists(localFile) {
		removeFile(localFile)
	}
	// The location to the remote url.
	remoteURL := []string{
		"https://cam2.com/data-sheets/",
		"https://cam2.com/product/cam2-premium-synthetic-blend-tc-w3-2-cycle-outboard-oil/",
		"https://cam2.com/product/cam2-magnum-economy-2-cycle-engine-oil/",
		"https://cam2.com/product/cam2-2-cycle-engine-oil-air-cooled/",
		"https://cam2.com/product/cam2-blue-blood-12-2-6-ounce-2-cycle-synthetic-engine-oil/",
		"https://cam2.com/product/cam2-blue-blood-marine-2-cycle-oil-tc-w3/",
		"https://cam2.com/product/cam2-nitrile-gloves-8mil-black-medium/",
		"https://cam2.com/product/cam2-promax-r-o-hydraulic-oil/",
		"https://cam2.com/product/cam2-full-synthetic-global-low-vis-atf/",
		"https://cam2.com/product/cam2-synavex-dexos1-gen-3-sae-5w-30-sp-gf-6a-full-synthetic-motor-oil/",
		"https://cam2.com/product/cam2-synavex-dexos1-gen-3-sae-0w-20-sp-gf-6a-full-synthetic-motor-oil/",
		"https://cam2.com/product/cam2-magnum-gear-oil-sae-80w-90-gl-5/",
		"https://cam2.com/product/cam2-magnum-gear-oil-sae-75w-140-gl-5/",
		"https://cam2.com/product/cam2-magnum-gear-oil-sae-75w-90-gl-5/",
		"https://cam2.com/product/cam2-k-1-kerosene/",
		"https://cam2.com/product/hand-sanitizer/",
		"https://cam2.com/product/cam2-blue-blood-elite-hd-5w-40-ck-4-w-detox-technology/",
		"https://cam2.com/product/cam2-ngeo-sae-15w-40-ces-20074-engine-oil/",
		"https://cam2.com/product/cam2-geo-sae-30-ashless-engine-oil/",
		"https://cam2.com/product/cam2-ngeo-low-ash-engine-oil-sae-30/",
		"https://cam2.com/product/cam2-ngeo-low-ash-engine-oil-sae-40/",
		"https://cam2.com/product/cam2-super-hd-15w-40-performance-driven-ck-4-sn-synthetic-blend-engine-oil/",
		"https://cam2.com/product/cam-2-super-hd-10w-30-performance-driven-ck-4-synthetic-blend-engine-oil/",
		"https://cam2.com/product/cam2-super-hd-sae-50-api-cf-cf-2-sl-engine-oil/",
		"https://cam2.com/product/cam2-super-hd-sae-40-api-cf-cf-2-sl-engine-oil/",
		"https://cam2.com/product/cam2-super-hd-sae-30-api-cf-cf-2-sl-engine-oil/",
		"https://cam2.com/product/cam2-super-hd-10w-40-ck-4-heavy-duty-engine-oil/",
		"https://cam2.com/product/cam2-super-hd-sae-10w-engine-oil/",
		"https://cam2.com/product/cam2-magnum-turbo-d-25w-60-ch-4-sg-green-with-tackifier-engine-oil/",
		"https://cam2.com/product/cam2-magnum-turbo-d-25w-50-ch-4-sg-engine-oil/",
		"https://cam2.com/product/cam2-magnum-turbo-d-20w-50-ci-4-plus-sl-engine-oil/",
		"https://cam2.com/product/cam2-magnum-turbo-d-20w-50-ch-4-sg-engine-oil/",
		"https://cam2.com/product/cam2-magnum-turbo-d-15w-40-ci-4-plus-sl-engine-oil/",
		"https://cam2.com/product/cam2-magnum-turbo-d-15w-40-ch-4-sg-engine-oil/",
		"https://cam2.com/product/cam2-s-k-railroad-engine-oil-9-tbn-sae-40/",
		"https://cam2.com/product/cam2-s-k-railroad-engine-oil-multigrade-9-tbn-20w-40/",
		"https://cam2.com/product/cam2-protect75-5w-30-sp-gf-6a-high-mileage-engine-oil/",
		"https://cam2.com/product/cam2-protect75-5w-20-sp-gf-6a-high-mileage-engine-oil/",
		"https://cam2.com/product/cam2-protect75-10w-40-sp-high-mileage-engine-oil/",
		"https://cam2.com/product/cam2-protect75-10w-30-sp-gf-6a-high-mileage-engine-oil/",
		"https://cam2.com/product/cam2-magnum-special-5w-20-synthetic-blend-engine-oil/",
		"https://cam2.com/product/ca2-magnum-special-5w-30-synthetic-blend-engine-oil/",
		"https://cam2.com/product/cam2-magnum-special-20w-50-synthetic-blend-engine-oil/",
		"https://cam2.com/product/cam2-magnum-special-10w-40-synthetic-blend-engine-oil/",
		"https://cam2.com/product/cam2-magnum-special-10w-30-synthetic-blend-engine-oil/",
		"https://cam2.com/product/cam2-nd-sae-50-motor-oil/",
		"https://cam2.com/product/cam2-nd-sae-40-motor-oil/",
		"https://cam2.com/product/cam2-nd-sae-30-motor-oil/",
		"https://cam2.com/product/cam2-nd-sae-20-motor-oil/",
		"https://cam2.com/product/cam2-nd-sae-10-motor-oil/",
		"https://cam2.com/product/cam2-magnum-sae-50-motor-oil/",
		"https://cam2.com/product/cam2-magnum-sae-40-motor-oil/",
		"https://cam2.com/product/cam2-superpro-max-30w-sp-synthetic-blend-motor-oil/",
		"https://cam2.com/product/cam2-superpro-max-40w-sp-synthetic-blend-motor-oil/",
		"https://cam2.com/product/cam2-superpro-max-20w-50-sp-synthetic-blend-motor-oil/",
		"https://cam2.com/product/cam2-superpro-max-10w-30-sp-synthetic-blend-motor-oil/",
		"https://cam2.com/product/cam2-superpro-max-10w-40-sp-synthetic-blend-motor-oil/",
		"https://cam2.com/product/cam2-superpro-max-5w-30-sp-synthetic-blend-motor-oil/",
		"https://cam2.com/product/cam2-superpro-max-5w-20-sp-synthetic-blend-motor-oil/",
		"https://cam2.com/product/cam2-blue-blood-nitro-70-synthetic-blend-racing-engine-oil/",
		"https://cam2.com/product/cam2-blue-blood-20w-50-synthetic-blend-racing-engine-oil/",
		"https://cam2.com/product/cam2-blue-blood-0w-30-full-synthetic-racing-engine-oil/",
		"https://cam2.com/product/cam2-blue-blood-high-performance-break-in-engine-oil/",
		"https://cam2.com/product/cam2-blue-blood-elite-0w-30-sp-gf-6a-full-synthetic-engine-oil/",
		"https://cam2.com/product/cam2-blue-blood-elite-0w-40-sp-full-synthetic-engine-oil/",
		"https://cam2.com/product/cam2-blue-blood-elite-10w-30-sp-gf-6a-full-synthetic-engine-oil/",
		"https://cam2.com/product/cam2-blue-blood-elite-5w-30-sp-gf-6a-full-synthetic-engine-oil/",
		"https://cam2.com/product/cam2-blue-blood-elite-5w-40-sp-full-synthetic-engine-oil/",
		"https://cam2.com/product/cam2-blue-blood-elite-euro-5w-30-full-synthetic-engine-oil/",
		"https://cam2.com/product/cam2-blue-blood-elite-euro-5w-40-full-synthetic-engine-oil/",
		"https://cam2.com/product/cam2-synavex-0w-16-sp-gf-6b-full-synthetic-engine-oil/",
		"https://cam2.com/product/cam2-synavex-0w-20-sp-gf-6a-full-synthetic-engine-oil/",
		"https://cam2.com/product/cam2-synavex-0w-40-sp-full-synthetic-engine-oil/",
		"https://cam2.com/product/cam2-synavex-10w-30-sp-gf-6a-full-synthetic-engine-oil/",
		"https://cam2.com/product/cam2-synavex-5w-20-sp-gf-6a-full-synthetic-engine-oil/",
		"https://cam2.com/product/cam2-synavex-5w-30-sp-gf-6a-full-synthetic-engine-oil/",
		"https://cam2.com/product/cam2-synavex-5w-40-sp-full-synthetic-engine-oil/",
		"https://cam2.com/product/magnum-special-multi-purpose-dexron-iii-mercon-atf/",
		"https://cam2.com/product/cam2-synavex-hd-trans-full-synthetic-transmission-fluid/",
		"https://cam2.com/product/cam2-synavex-full-synthetic-trans-fluid-sae-40/",
		"https://cam2.com/product/cam2-full-synthetic-cvt-transmission-fluid/",
		"https://cam2.com/product/cam2-synavex-full-synthetic-sae-50-transmission-fluid/",
		"https://cam2.com/product/cam2-dexron-vi-multi-vehicle-full-synthetic-atf/",
		"https://cam2.com/product/cam2-mpt-sae-50-torque-fluid-to-4/",
		"https://cam2.com/product/cam2-mpt-sae-30-torque-fluid-to-4/",
		"https://cam2.com/product/cam2-mpt-sae-10w-torque-fluid-to-4/",
		"https://cam2.com/product/cam2-mercon-v-multi-purpose-atf/",
		"https://cam2.com/product/cam2-type-f-atf/",
		"https://cam2.com/product/cam2-atf-d-m-dexron-iiih-mercon/",
		"https://cam2.com/product/cam2-atf-4/",
		"https://cam2.com/product/cam2-multi-vehicle-synthetic-blend-atf/",
		"https://cam2.com/product/cam2-type-a-atf/",
		"https://cam2.com/product/cam2-85w-140-high-performance-ep-gear-oil-gl-5/",
		"https://cam2.com/product/cam2-80w-90-ls-gear-oil-gl-5/",
		"https://cam2.com/product/cam2-80w-90-high-performance-ep-gear-oil-gl-5/",
		"https://cam2.com/product/cam2-industrial-ep-gear-oil-680/",
		"https://cam2.com/product/cam2-industrial-ep-gear-oil-32/",
		"https://cam2.com/product/cam2-industrial-ep-gear-oil-68/",
		"https://cam2.com/product/cam2-industrial-ep-gear-oil-460/",
		"https://cam2.com/product/cam2-industrial-ep-gear-oil-320/",
		"https://cam2.com/product/cam2-industrial-ep-gear-oil-220/",
		"https://cam2.com/product/cam2-industrial-ep-gear-oil-150/",
		"https://cam2.com/product/cam2-industrial-ep-gear-oil-100/",
		"https://cam2.com/product/cam2-ep-320-synthetic-industrial-gear-oil/",
		"https://cam2.com/product/cam2-ep-220-synthetic-industrial-gear-oil/",
		"https://cam2.com/product/cam2-ep-150-synthetic-industrial-gear-oil/",
		"https://cam2.com/product/magnum-industrial-gear-oil-ep-220/",
		"https://cam2.com/product/magnum-industrial-gear-oil-ep-460/",
		"https://cam2.com/product/cam2-magnum-gear-oil-90-gl-1/",
		"https://cam2.com/product/cam2-magnum-gear-oil-140-gl-1/",
		"https://cam2.com/product/cam2-blue-blood-80w-90-ls-gear-oil-gl-5/",
		"https://cam2.com/product/cam2-synavex-full-synthetic-80w-140-ls-gear-oil/",
		"https://cam2.com/product/cam2-synavex-full-synthetic-75w-90-ls-gear-oil/",
		"https://cam2.com/product/cam2-synavex-full-synthetic-75w-140-ls-gear-oil/",
		"https://cam2.com/product/cam2-blue-blood-sae-75w-90-full-synthetic-ls-gear-oil-gl-5/",
		"https://cam2.com/product/cam2-ashless-aw-68-hydraulic-oil/",
		"https://cam2.com/product/cam2-ashless-aw-46-hydraulic-oil/",
		"https://cam2.com/product/cam2-ashless-aw-32-hydraulic-oil/",
		"https://cam2.com/product/cam2-promax-premium-aw-150-hydraulic-oil/",
		"https://cam2.com/product/cam2-promax-premium-aw-10-low-temp-hydraulic-oil/",
		"https://cam2.com/product/cam2-promax-premium-aw-15-low-temp-hydraulic-oil/",
		"https://cam2.com/product/cam2-promax-premium-aw-22-hydraulic-oil/",
		"https://cam2.com/product/cam2-promax-premium-aw-68-hydraulic-oil/",
		"https://cam2.com/product/cam2-promax-premium-aw-46-hydraulic-oil/",
		"https://cam2.com/product/cam2-promax-premium-aw-32-hydraulic-oil/",
		"https://cam2.com/product/cam2-promax-premium-aw-100-hydraulic-oil/",
		"https://cam2.com/product/cam2-promax-premium-all-season-5w-20-hydraulic-oil/",
		"https://cam2.com/product/cam2-sae-20w-hydra-cat-1000-hydraulic-fluid/",
		"https://cam2.com/product/cam2-sae-10w-hydra-cat-1000-hydraulic-fluid/",
		"https://cam2.com/product/cam2-mining-hydraulic-68-fluid/",
		"https://cam2.com/product/cam2-promax-aw-150-hydraulic-oil/",
		"https://cam2.com/product/cam-2-promax-aw-100-hydraulic-oil/",
		"https://cam2.com/product/cam2-promax-aw-15-hydraulic-oil/",
		"https://cam2.com/product/cam2-promax-aw-22-hydraulic-fluid/",
		"https://cam2.com/product/cam2-promax-aw-68-hydraulic-oil/",
		"https://cam2.com/product/cam2-promax-aw-46-hydraulic-oil/",
		"https://cam2.com/product/cam2-promax-aw-32-hydraulic-oil/",
		"https://cam2.com/product/cam2-promax-tractor-hydraulic-fluid-j20-d/",
		"https://cam2.com/product/cam2-promax-premium-universal-tractor-hydraulic-fluid/",
		"https://cam2.com/product/cam2-ag-20-hydraulic-fluid/",
		"https://cam2.com/product/cam2-synthetic-air-compressor-oil-68/",
		"https://cam2.com/product/cam2-synthetic-air-compressor-oil-46/",
		"https://cam2.com/product/cam2-synthetic-air-compressor-oil-32/",
		"https://cam2.com/product/cam-2-heat-transfer-oil-iso-150/",
		"https://cam2.com/product/cam-2-heat-transfer-oil-iso-46/",
		"https://cam2.com/product/cam-2-heat-transfer-oil-iso-32/",
		"https://cam2.com/product/cam2-iso-32-synthetic-heat-transfer-oil/",
		"https://cam2.com/product/cam2-iso-46-synthetic-heat-transfer-oil/",
		"https://cam2.com/product/cam2-iso-68-synthetic-heat-transfer-oil/",
		"https://cam2.com/product/cam2-rock-drill-oil-320/",
		"https://cam2.com/product/cam2-rock-drill-oil-220/",
		"https://cam2.com/product/cam2-rock-drill-oil-100/",
		"https://cam2.com/product/cam2-ultra-turbine-oil-68/",
		"https://cam2.com/product/cam2-ultra-turbine-oil-46/",
		"https://cam2.com/product/cam2-ultra-turbine-oil-320/",
		"https://cam2.com/product/cam2-ultra-turbine-oil-32/",
		"https://cam2.com/product/cam2-ultra-turbine-oil-220/",
		"https://cam2.com/product/cam2-ultra-turbine-oil-22/",
		"https://cam2.com/product/cam2-ultra-turbine-oil-150/",
		"https://cam2.com/product/cam2-ultra-turbine-oil-100/",
		"https://cam2.com/product/cam2-ultra-turbine-oil-460/",
		"https://cam2.com/product/cam2-way-lube-460/",
		"https://cam2.com/product/cam2-way-lube-150/",
		"https://cam2.com/product/cam2-way-lube-100/",
		"https://cam2.com/product/cam2-way-lube-68/",
		"https://cam2.com/product/cam2-way-lube-32/",
		"https://cam2.com/product/cam2-way-lube-220/",
		"https://cam2.com/product/cam2-wireseal-2500/",
		"https://cam2.com/product/cam2-wireseal-1500/",
		"https://cam2.com/product/cam2-wireseal-680/",
		"https://cam2.com/product/cam2-cherry-picker-oil-iso-32/",
		"https://cam2.com/product/cam2-cherry-picker-oil-iso-22/",
		"https://cam2.com/product/cam2-aviation-smoke-oil/",
		"https://cam2.com/product/cam2-drip-oil/",
		"https://cam2.com/product/cam2-concrete-form-oil/",
		"https://cam2.com/product/cam2-saw-guide-oil-150/",
		"https://cam2.com/product/cam2-saw-guide-oil-100/",
		"https://cam2.com/product/cam2-ultraplex-ep1-grease-with-moly-graphite/",
		"https://cam2.com/product/cam2-ultraplex-ep2-grease-lithium-complex-with-2-moly/",
		"https://cam2.com/product/cam2-ultraplex-ep-2-grease-lithium-complex-with-3-moly/",
		"https://cam2.com/product/cam2-ultraplex-ep-2-hi-temp-lithium-complex-grease/",
		"https://cam2.com/product/cam2-hi-temp-red-lithium-complex-grease/",
		"https://cam2.com/product/cam2-cotton-picker-spindle-grease/",
		"https://cam2.com/product/cam2-multi-purpose-lithium-grease/",
		"https://cam2.com/product/cam2-ultra580-ep-2-grease-calcium-sulfonate-with-5-moly/",
		"https://cam2.com/product/cam2-ultra-580-ep2-grease/",
		"https://cam2.com/product/cam2-ultra-580-ep1-grease/",
		"https://cam2.com/product/cam2-transformer-oil/",
		"https://cam2.com/product/cam2-60-pale-oil/",
		"https://cam2.com/product/cam-2-hvi-325-base-oil/",
		"https://cam2.com/product/cam-2-hvi-240-base-oil/",
		"https://cam2.com/product/cam-2-hvi-150-base-oil/",
		"https://cam2.com/product/cam-2-hvi-120-base-oil/",
		"https://cam2.com/product/cam-2-hvi-70-base-oil/",
		"https://cam2.com/product/cam2-conventional-pre-mix-50-50-antifreeze-coolant/",
		"https://cam2.com/product/cam2-conventional-full-strength-antifreeze-coolant/",
		"https://cam2.com/product/cam2-global-pre-mix-50-50-antifreeze/",
		"https://cam2.com/product/cam2-global-full-strength-antifreeze/",
		"https://cam2.com/product/cam2-superlife-pre-mix-50-50-antifreeze/",
		"https://cam2.com/product/cam2-superlife-full-strength-antifreeze/",
		"https://cam2.com/product/cam2-superlife-fleet-hd-truck-full-strength-antifreeze/",
		"https://cam2.com/product/cam2-superlife-fleet-hd-truck-pre-mix-50-50-antifreeze/",
		"https://cam2.com/product/cam2-magnum-radiator-additive/",
		"https://cam2.com/product/cam2-non-flammable-flat-tire-sealant-hose/",
		"https://cam2.com/product/cam2-non-flammable-flat-tire-sealant-cone/",
		"https://cam2.com/product/cam2-penetrating-oil/",
		"https://cam2.com/product/cam2-carburetor-cleaner/",
		"https://cam2.com/product/cam2-de-icer/",
		"https://cam2.com/product/cam2-starting-fluid/",
		"https://cam2.com/product/cam2-super-hd-brake-parts-cleaner-non-flammable/",
		"https://cam2.com/product/cam2-super-hd-brake-parts-cleaner-non-chlorinated/",
		"https://cam2.com/product/cam2-loggers-pride-no-sling-premium-bar-chain-oil/",
		"https://cam2.com/product/cam2-all-season-low-sling-bar-chain-oil/",
		"https://cam2.com/product/cam2-blue-blood-def/",
		"https://cam2.com/product/cam2-cleaner-degreaser/",
		"https://cam2.com/product/cam2-140-high-flash-odorless-mineral-spirits/",
		"https://cam2.com/product/cam2-oil-treatment/",
		"https://cam2.com/product/cam2-motor-sealer/",
		"https://cam2.com/product/cam2-power-steering-motor-sealer/",
		"https://cam2.com/product/cam-2-diesel-conditioner-anti-gel/",
		"https://cam2.com/product/cam2-octane-booster/",
		"https://cam2.com/product/cam2-gas-treatment/",
		"https://cam2.com/product/cam2-fuel-storage-stabilizer/",
		"https://cam2.com/product/cam2-carb-fuel-injector-cleaner/",
		"https://cam2.com/product/cam2-power-steering-fluid/",
		"https://cam2.com/product/cam2-super-hd-brake-fluid-dot-3/",
		"https://cam2.com/product/cam2-super-hd-brake-fluid-dot4/",
		"https://cam2.com/product/cam2-windshield-washer-concentrate/",
		"https://cam2.com/product/cam2-charcoal-lighter-fluid/",
		"https://cam2.com/product/cam2-aluminum-brightener-fiberglass-cleaner/",
		"https://cam2.com/product/cam2-cotton-picker-spindle-cleaner/",
		"https://cam2.com/product/cam2-blue-blood-elite-4t-10w-40-synthetic-motorcycle-oil/",
		"https://cam2.com/product/cam2-blue-blood-elite-4t-20w-50-synthetic-motorcycle-oil/",
	}
	// Loop over the urls and save content to file.
	for _, url := range remoteURL {
		// Call fetchPage to download the content of that page
		pageContent := getDataFromURL(url)
		// Append it and save it to the file.
		appendAndWriteToFile(localFile, pageContent)
	}
	// Read the file content
	fileContent := readAFileAsString(localFile)
	// Extract the URLs from the given content.
	extractedPDFURLs := extractPDFUrls(fileContent)
	// Remove duplicates from the slice.
	extractedPDFURLs = removeDuplicatesFromSlice(extractedPDFURLs)
	// Loop through all extracted PDF URLs
	for _, urls := range extractedPDFURLs {
		if !hasDomain(urls) {
			urls = remoteDomainName + urls

		}
		if isUrlValid(urls) { // Check if the final URL is valid
			downloadPDF(urls, outputDir) // Download the PDF
		}
	}
}
