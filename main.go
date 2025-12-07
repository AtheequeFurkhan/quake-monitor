package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

// --- CONFIGURATION ---
const (
	USGS_URL  = "https://earthquake.usgs.gov/earthquakes/feed/v1.0/summary/all_hour.geojson"
	NASA_URL  = "https://eonet.gsfc.nasa.gov/api/v3/events?status=open&limit=10" // Top 10 recent active events
)

// --- STRUCTS: EARTHQUAKE (USGS) ---
type QuakeCollection struct {
	Metadata QuakeMetadata `json:"metadata"`
	Features []QuakeFeature `json:"features"`
}
type QuakeMetadata struct {
	Count int    `json:"count"`
	Url   string `json:"url"`
}
type QuakeFeature struct {
	Properties QuakeProps `json:"properties"`
}
type QuakeProps struct {
	Mag   float64 `json:"mag"`
	Place string  `json:"place"`
	Time  int64   `json:"time"`
	Url   string  `json:"url"`
}

// --- STRUCTS: NASA EONET (Wildfires/Volcanoes) ---
type EONETResponse struct {
	Events []EONETEvent `json:"events"`
}
type EONETEvent struct {
	Title      string        `json:"title"`
	Categories []EONETCategory `json:"categories"`
	Sources    []EONETSource   `json:"sources"`
	Geometry   []EONETGeo      `json:"geometry"`
}
type EONETCategory struct {
	Id    string `json:"id"`
	Title string `json:"title"`
}
type EONETSource struct {
	Id  string `json:"id"`
	Url string `json:"url"`
}
type EONETGeo struct {
	Date string `json:"date"`
}

// --- MAIN EXECUTION ---
func main() {
	start := time.Now()

	// We use a WaitGroup to fetch both APIs at the same time (Concurrency)
	var wg sync.WaitGroup
	wg.Add(2)

	var quakeData *QuakeCollection
	var nasaData *EONETResponse
	var err1, err2 error

	// 1. Fetch Earthquakes (Goroutine)
	go func() {
		defer wg.Done()
		quakeData, err1 = fetchQuakes()
	}()

	// 2. Fetch NASA Data (Goroutine)
	go func() {
		defer wg.Done()
		nasaData, err2 = fetchNASA()
	}()

	wg.Wait() // Wait for both to finish

	// Error handling (Don't crash if one fails, just warn)
	if err1 != nil {
		fmt.Printf("Warning: Failed to fetch Quakes: %v\n", err1)
		quakeData = &QuakeCollection{} // Empty struct
	}
	if err2 != nil {
		fmt.Printf("Warning: Failed to fetch NASA: %v\n", err2)
		nasaData = &EONETResponse{} // Empty struct
	}

	// 3. Generate and Write Markdown
	markdown := generateMarkdown(quakeData, nasaData, time.Since(start))
	err := os.WriteFile("README.md", []byte(markdown), 0644)
	if err != nil {
		log.Fatalf("Error writing README: %v", err)
	}

	fmt.Println("Global Disaster Monitor updated successfully.")
}

// --- FETCH FUNCTIONS ---

func fetchQuakes() (*QuakeCollection, error) {
	resp, err := http.Get(USGS_URL)
	if err != nil { return nil, err }
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var data QuakeCollection
	if err := json.Unmarshal(body, &data); err != nil { return nil, err }
	return &data, nil
}

func fetchNASA() (*EONETResponse, error) {
	client := http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(NASA_URL)
	if err != nil { return nil, err }
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var data EONETResponse
	if err := json.Unmarshal(body, &data); err != nil { return nil, err }
	return &data, nil
}

// --- MARKDOWN GENERATION ---

func generateMarkdown(quakes *QuakeCollection, nasa *EONETResponse, duration time.Duration) string {
	now := time.Now().UTC().Format(time.RFC1123)
	var sb strings.Builder

	// HEADER
	sb.WriteString("# 🌪️ Global Disaster Monitor\n\n")
	sb.WriteString("> *Real-time tracking of Earthquakes, Wildfires, and Volcanoes using Go & GitHub Actions.*\n\n")
	
	// BADGES
	sb.WriteString(fmt.Sprintf("![Last Updated](https://img.shields.io/badge/Updated-%s-blue) ", strings.ReplaceAll(now, " ", "%20")))
	sb.WriteString(fmt.Sprintf("![Build Time](https://img.shields.io/badge/Build%%20Time-%s-green) ", duration.Round(time.Millisecond)))
	sb.WriteString("![System](https://img.shields.io/badge/System-Operational-success)\n\n")

	// SECTION 1: EARTHQUAKES
	sb.WriteString("## 📉 Earthquakes (Last Hour)\n")
	if len(quakes.Features) == 0 {
		sb.WriteString("✅ *No significant seismic activity recorded in the past hour.*\n")
	} else {
		sb.WriteString("| Mag | Location | Time (UTC) |\n")
		sb.WriteString("|:---:|:---|:---|\n")
		for _, f := range quakes.Features {
			t := time.Unix(f.Properties.Time/1000, 0).UTC().Format("15:04")
			
			// Danger Icons
			icon := "🟢"
			if f.Properties.Mag > 5.0 { icon = "🔴" } else if f.Properties.Mag > 3.0 { icon = "🟠" }

			sb.WriteString(fmt.Sprintf("| %s %.1f | %s | %s |\n", icon, f.Properties.Mag, f.Properties.Place, t))
		}
	}

	// SECTION 2: NASA EVENTS
	sb.WriteString("\n## 🌋 Active Hazards (NASA EONET)\n")
	sb.WriteString("*Includes Wildfires, Volcanoes, and Icebergs detected by satellite.*\n\n")

	if len(nasa.Events) == 0 {
		sb.WriteString("✅ *No active major hazards reported by NASA right now.*\n")
	} else {
		for _, e := range nasa.Events {
			// Get Category Icon
			catIcon := "⚠️"
			catName := "Unknown"
			if len(e.Categories) > 0 {
				catName = e.Categories[0].Title
				switch e.Categories[0].Id {
				case "wildfires": catIcon = "🔥"
				case "volcanoes": catIcon = "🌋"
				case "severeStorms": catIcon = "⛈️"
				case "seaLakeIce": catIcon = "❄️"
				}
			}
			
			// Get Date
			date := "N/A"
			if len(e.Geometry) > 0 {
				// Parse NASA date (2023-10-05T00:00:00Z)
				parsedTime, _ := time.Parse(time.RFC3339, e.Geometry[len(e.Geometry)-1].Date)
				date = parsedTime.Format("Jan 02")
			}
			
			// Get Link
			link := "#"
			if len(e.Sources) > 0 { link = e.Sources[0].Url }

			sb.WriteString(fmt.Sprintf("- %s **%s**: [%s](%s) (%s)\n", catIcon, catName, e.Title, link, date))
		}
	}

	sb.WriteString("\n---\n")
	sb.WriteString("Generated automatically by [GitHub Actions](https://github.com/features/actions) running a [Go](https://go.dev/) script.\n")

	return sb.String()
}