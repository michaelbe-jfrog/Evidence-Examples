package main
import (
    "context"
    "encoding/json"
    "os"
    "fmt"
    "io"
    "log"
	"net/http"
	"time"
    "bufio"
    "strings"
  )
  /*
    SonarScanResponse is the json analysis that was outputed by sonar
    and will be returned to the calling build process for cresting an evidence

   notice that the calling client should first check that return value was 0 before using the response JSON,
   otherwise the response is an error message which cannot be parsed
   */
// sonar response struct
type SonarResponse struct {
    Task SonarTask `json:"task"`
    Analysis ProjectStatus `json:"analysis"`
}

// Define a struct to hold the response data
type SonarTaskResponse struct {
	Task SonarTask `json:"task"`
}

type SonarTask struct {
	    id string `json:"id"`
		Status string `json:"status"`
		AnalysisId string `json:"analysisId"`
		ComponentId string `json:"componentId"`
		ComponentKey string `json:"componentKey"`
		ComponentName string `json:"componentName"`
		Organization string `json:"organization"`
		SubmittedAt string `json:"submittedAt"`
		SubmitterLogin string `json:"submitterLogin"`
		StartedAt string `json:"startedAt"`
		ExecutedAt string `json:"executedAt"`
	}
type Condition struct {
	Status         string `json:"status"`
	MetricKey      string `json:"metricKey"`
	Comparator     string `json:"comparator"`
	PeriodIndex    int    `json:"periodIndex"`
	ErrorThreshold string `json:"errorThreshold"`
	ActualValue    string `json:"actualValue"`
}

type Period struct {
	Index int    `json:"index"`
	Mode  string `json:"mode"`
	Date  string `json:"date"`
}

type ProjectStatus struct {
	Status            string      `json:"status"`
	Conditions        []Condition `json:"conditions"`
	Periods           []Period    `json:"periods"`
	IgnoredConditions bool        `json:"ignoredConditions"`
}

type SonarAnalysis struct {
	ProjectStatus ProjectStatus `json:"projectStatus"`
}

const (
	DEFAULT_HTTP_TIMEOUT = 10 * time.Second
	ANALYSIS_URL = "https://sonarcloud.io/api/qualitygates/project_status?analysisId=$analysisId"
    LOG_FILE_LOCATION = "sonar-scan.log"
)
func main() {
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	logFile, err := os.OpenFile(LOG_FILE_LOCATION, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatal(err)
	}
	defer logFile.Close()
	logger := log.New(logFile, "[SONAR EVIDENCE CREATION] ", log.Ldate|log.Ltime|log.Lshortfile)
    logger.Println("Running sonar analysis extraction")
    sonar_token := os.Getenv("SONAR_TOKEN")
    if sonar_token == "" {
        logger.Println("Sonar token not found, set SONAR_TOKEN variable")
        os.Exit(1)
    }
///home/runner/work/Evidence-Examples/Evidence-Examples/.scannerwork/report-task.txt
    //get the sonar report file location or details to .scannerwork/.report-task.txt
    reportTaskFile := ".scannerwork/.report-task.txt"
    if len(os.Args) > 0 {
        reportTaskFile = os.Args[1]
     }
    // fmt.Println("reportTaskFile: ", reportTaskFile)
    // Open the reportTaskFile
	file, err := os.Open(reportTaskFile)
	if err != nil {
		logger.Println("Error opening file:", err)
		return
	}
	defer file.Close()
	ceTaskUrl:=""

	// Read the file line by line
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		// Skip empty lines and comments
		if len(line) == 0 || strings.HasPrefix(line, "#") {
			continue
		}
		// Split the line into key and value
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			if key == "ceTaskUrl" {
			    ceTaskUrl = value
			    break
			}
		}
	}
	if err := scanner.Err(); err != nil {
		logger.Println("Error reading file:", reportTaskFile , "error", err)
        os.Exit(1)
	}
	if ceTaskUrl == "" {
		fmt.Printf("ceTaskUrl Key not found")
		os.Exit(1)
	}
    // Add a reusable HTTP client
	var client = &http.Client{
		Timeout: DEFAULT_HTTP_TIMEOUT,
		Transport: &http.Transport{
			MaxIdleConns:       100,
			IdleConnTimeout:    10 * time.Second,
			DisableCompression: true,
		},
	}
	logger.Println("ceTaskUrl", ceTaskUrl)
    taskResponse, err := getReport(ctx, client, logger, ceTaskUrl, sonar_token )
    if err != nil {
        logger.Println("Error getting sonar report task", err)
        os.Exit(1)
    }
	logger.Println("taskResponse.Task.AnalysisId", taskResponse.Task.AnalysisId)
	logger.Println("taskResponse.Task.id", taskResponse.Task.id)

    // get the analysis content
    analysis , err := getAnalysis(ctx, client, logger, sonar_token, taskResponse.Task.AnalysisId)
    if err != nil {
        logger.Println("Error getting sonar analysis report: ", err)
        os.Exit(1)
    }

    response := SonarResponse{
        Task: taskResponse.Task,
        Analysis: analysis.ProjectStatus,
    }

    // marshal the response to JSON
	jsonBytes, err := json.Marshal(response)
	if err != nil {
		logger.Println("Error marshaling JSON", err)
		os.Exit(1)
	}
	// return response to caller through stdout
	os.Stdout.Write(jsonBytes)
 }


func getReport(ctx context.Context , client *http.Client, logger *log.Logger, ceTaskUrl string, sonar_token string) (SonarTaskResponse, error) {
	 // Make the HTTP GET request
	logger.Println("getReport ceTaskUrl:",ceTaskUrl)
	req, err := http.NewRequestWithContext(ctx, "GET", ceTaskUrl, nil)
	req.Header.Set("Authorization", "Bearer " + sonar_token)
	resp, err := client.Do(req)
	if err != nil {
		return SonarTaskResponse{}, fmt.Errorf("Error making the request, url:",ceTaskUrl, "error", err)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return SonarTaskResponse{},  fmt.Errorf("getReport error readinf body error:",err)
	}

	if resp.StatusCode != http.StatusOK {
		return SonarTaskResponse{}, fmt.Errorf("getReport error getting response from", ceTaskUrl, " returned ", resp.StatusCode, " response body ", body)
	}

    logger.Println("getReport resp.StatusCode:", resp.StatusCode)

    defer func(Body io.ReadCloser) {
        err := Body.Close()
        if err != nil {
          logger.Println("Error reding SonarTaskResponse, error:", err)
            // not printing an error so stdout is not effected
        }
	}(resp.Body)



    var taskResponse SonarTaskResponse
    err = json.Unmarshal(body, &taskResponse)
    if err != nil {
		log.Println("get temp credentials response body ", string(body))
		return SonarTaskResponse{}, fmt.Errorf("error decoding report response for report", ceTaskUrl, "error",  err)
	}
	return taskResponse, nil
}

func getAnalysis(ctx context.Context, client *http.Client, logger *log.Logger, sonar_token string, analysisId string) (SonarAnalysis, error) {

    analysisUrl := strings.Replace(ANALYSIS_URL, "$analysisId", analysisId, 1)
    logger.Println("analysisId", analysisId)
    //logger.Println("analysisUrl", analysisUrl)
	 // Make the HTTP GET request
	req, err := http.NewRequestWithContext(ctx, "GET", analysisUrl , nil)
	req.Header.Set("Authorization", "Bearer " + sonar_token)
	resp, err := client.Do(req)
	if err != nil {
		return SonarAnalysis{}, fmt.Errorf("Error making the request, url:",analysisUrl, "error", err)
	}
    defer func(Body io.ReadCloser) {
        err := Body.Close()
        if err != nil {
            // not printing an error so stdout is not effected
        }
	}(resp.Body)

	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		return SonarAnalysis{}, fmt.Errorf("error getting response from", analysisUrl, " returned ", resp.StatusCode, " response body ", body, "error", err)
	}

    analysisResponse := &SonarAnalysis{}
	err = json.NewDecoder(resp.Body).Decode(analysisResponse)
	if err != nil {
		return SonarAnalysis{}, fmt.Errorf("error decoding report response for report", analysisUrl, "error",  err)
	}

	return *analysisResponse, nil
}