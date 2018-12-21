package main

import (
	"encoding/xml"
	"io/ioutil"
	"log"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/jlaffaye/ftp"
	"github.com/sukhjit/bom-weather-api/forecast"
)

const (
	timeLayout        = "2006-01-02T15:04:05-07:00"
	dateFormat        = "2006-01-02"
	ftpServerHost     = "ftp.bom.gov.au"
	ftpServerPort     = "21"
	ftpServerUsername = "anonymous"
	ftpServerPassword = "password"
)

type stateXML struct {
	XMLNAME        xml.Name         `xml:"product"`
	IssueTimeLocal string           `xml:"amoc>issue-time-local"`
	IssueTimeUtc   string           `xml:"amoc>issue-time-utc"`
	Forecast       stateForecastXML `xml:"forecast"`
}

type stateForecastXML struct {
	XMLNAME xml.Name  `xml:"forecast"`
	Area    []areaXML `xml:"area"`
}

type areaXML struct {
	XMLNAME        xml.Name            `xml:"area"`
	Aac            string              `xml:"aac,attr"`
	Description    string              `xml:"description,attr"`
	Type           string              `xml:"type,attr"`
	ParentAac      string              `xml:"parent-aac,attr"`
	ForecastPeriod []forecastPeriodXML `xml:"forecast-period"`
}

type forecastPeriodXML struct {
	XMLNAME        xml.Name     `xml:"forecast-period"`
	Index          string       `xml:"index,attr"`
	StartTimeLocal string       `xml:"start-time-local,attr"`
	EndTimeLocal   string       `xml:"end-time-local,attr"`
	StartTimeUtc   string       `xml:"start-time-utc,attr"`
	EndTimeUtc     string       `xml:"end-time-utc,attr"`
	Elements       []elementXML `xml:"element"`
	Texts          []textXML    `xml:"text"`
}

type elementXML struct {
	XMLNAME string `xml:"element"`
	Type    string `xml:"type,attr"`
	Units   string `xml:"units,attr"`
	Value   string `xml:",chardata"`
}

type textXML struct {
	XMLNAME string `xml:"text"`
	Type    string `xml:"type,attr"`
	Value   string `xml:",chardata"`
}

func main() {
	lambda.Start(handler)
}

func handler() error {
	fileStateMapping := map[string]string{
		// processing nsw only for now, because of write limit of dynamodb
		// prefer to use sqs for inserts
		"nsw": "/anon/gen/fwo/IDN11060.xml",
		// "nt":  "/anon/gen/fwo/IDD10207.xml",
		// "qld": "/anon/gen/fwo/IDQ11295.xml",
		// "sa":  "/anon/gen/fwo/IDS10044.xml",
		// "tas": "/anon/gen/fwo/IDT16710.xml",
		// "vic": "/anon/gen/fwo/IDV10753.xml",
		// "wa":  "/anon/gen/fwo/IDW14199.xml",
	}

	finalList := []forecast.Forecast{}
	for state, filename := range fileStateMapping {
		log.Printf("State: %s, File: %s\n", state, filename)

		client, err := ftp.Dial(ftpServerHost + ":" + ftpServerPort)
		if err != nil {
			log.Printf("failed to dial: %v\n", err)
			continue
		}

		if err := client.Login(ftpServerUsername, ftpServerPassword); err != nil {
			log.Printf("failed to login: %v\n", err)
			continue
		}

		reader, err := client.Retr(filename)
		if err != nil {
			log.Printf("ERROR: failed to retrieve file: %v\n", err)
			continue
		}
		defer reader.Close()

		buf, err := ioutil.ReadAll(reader)
		if err != nil {
			log.Printf("ERROR: failed to read file: %v\n", err)
			continue
		}

		list := extractFileContent(state, buf)

		counter := 0
		for _, row := range list {
			if row.Error != nil {
				log.Printf("ERROR: failed to process record for: %s, dated: %s, %v\n", row.Location, row.Date, err)
				continue
			}

			finalList = append(finalList, row)

			counter++
		}

		log.Printf("Processed %d records out of %d, for State: %s\n", counter, len(list), state)
	}

	today := time.Now()
	tomorrow := today.AddDate(0, 0, 1)
	tomorrowPlus := today.AddDate(0, 0, 2)

	tomorrowStr := tomorrow.Format(dateFormat)
	tomorrowPlusStr := tomorrowPlus.Format(dateFormat)

	counter := 0
	for _, row := range finalList {
		if row.Date != tomorrowStr && row.Date != tomorrowPlusStr {
			// only write tomorrow's and day after tomorrow's data, to keep writes to minimum
			continue
		}

		if err := forecast.SaveRecord(&row); err != nil {
			log.Printf("ERROR: failed to write record to dynamo db: %v\n", err)
			continue
		}

		counter++
	}

	log.Printf("Saved %d records out of %d to dynamodb\n", counter, len(finalList))

	return nil
}

func extractFileContent(stateName string, fileContent []byte) []forecast.Forecast {
	var stateXMLData stateXML
	xml.Unmarshal(fileContent, &stateXMLData)

	return mapXMLData(stateName, &stateXMLData)
}

func mapXMLData(stateName string, data *stateXML) []forecast.Forecast {
	list := []forecast.Forecast{}

	for _, area := range data.Forecast.Area {
		// skip area without location type, dont have any data
		if area.Type != "location" {
			continue
		}

		for _, forecastRow := range area.ForecastPeriod {
			asset := forecast.Forecast{
				Location: area.Description,
				State:    stateName,
			}

			startTimeLocal, err := convertStringToTime(forecastRow.StartTimeLocal)
			if err != nil {
				asset.Error = err
				list = append(list, asset)
				continue
			}

			asset.Date = startTimeLocal.Format(dateFormat)
			asset.SecondaryID = forecast.ConstructSecondaryID(asset.Location, asset.Date)

			// generating ID from location, date and state to overwrite existing item
			asset.ID = forecast.ComputeMainID(asset.Location, asset.Date, asset.State)
			for _, elementRow := range forecastRow.Elements {
				if elementRow.Type == "air_temperature_minimum" {
					asset.MinTemp = elementRow.Value
				}

				if elementRow.Type == "air_temperature_maximum" {
					asset.MaxTemp = elementRow.Value
				}
			}

			for _, textRow := range forecastRow.Texts {
				if textRow.Type == "precis" {
					asset.Precis = textRow.Value
				}

				if textRow.Type == "probability_of_precipitation" {
					asset.Precipitation = strings.Replace(textRow.Value, "%", "", 1)
				}
			}

			list = append(list, asset)
		}
	}

	return list
}

func convertStringToTime(str string) (time.Time, error) {
	t, err := time.Parse(timeLayout, str)

	return t, err
}
