package forecast

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/aws/aws-sdk-go/service/dynamodb/expression"
)

var db *dynamodb.DynamoDB

const (
	hmacSecret = "i-scream-you-scream"
	awsRegion  = "ap-southeast-2"
	tableName  = "bom-weather-api-WeatherDynamoTable-Z0NODNHHPWJ3"
)

// Forecast struct
type Forecast struct {
	ID            string `json:"id"`
	SecondaryID   string `json:"-"`
	Location      string `json:"location"`
	State         string `json:"state"`
	MinTemp       string `json:"minTemp"`
	MaxTemp       string `json:"maxTemp"`
	Precis        string `json:"precis"`
	Precipitation string `json:"precipitation"`
	Date          string `json:"date"`
	Error         error  `json:"-"`
}

func init() {
	sess := session.Must(session.NewSession(&aws.Config{
		Region: aws.String(awsRegion),
	}))

	db = dynamodb.New(sess)
}

// ComputeMainID from forecast struct
func ComputeMainID(location, date, state string) string {
	message := fmt.Sprintf("%s-%s-%s", location, date, state)
	key := []byte(hmacSecret)

	h := hmac.New(sha256.New, key)

	h.Write([]byte(message))

	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

// ConstructSecondaryID will create secondary ID from given params
func ConstructSecondaryID(location, date string) string {
	location = strings.ToLower(location)
	location = strings.Replace(location, " ", "-", -1)

	return fmt.Sprintf("%s-%s", date, location)
}

// GetItemsBySecondaryID will fetch items by secodary id
func GetItemsBySecondaryID(location, date string) ([]*Forecast, error) {
	sid := ConstructSecondaryID(location, date)

	filt := expression.Name("secondaryID").Equal(expression.Value(sid))

	expr, err := expression.NewBuilder().WithFilter(filt).Build()
	if err != nil {
		return nil, err
	}

	result, err := db.Scan(&dynamodb.ScanInput{
		TableName:                 aws.String(tableName),
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		FilterExpression:          expr.Filter(),
	})

	if err != nil {
		return nil, err
	}

	var items []*Forecast
	err = dynamodbattribute.UnmarshalListOfMaps(result.Items, &items)
	if err != nil {
		return nil, err
	}

	return items, nil
}

// GetItemsOlderThanDate return list of item older than given date
func GetItemsOlderThanDate(date string) ([]*Forecast, error) {
	filt := expression.Name("date").LessThan(expression.Value(date))

	expr, err := expression.NewBuilder().WithFilter(filt).Build()
	if err != nil {
		return nil, err
	}

	result, err := db.Scan(&dynamodb.ScanInput{
		TableName:                 aws.String(tableName),
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		FilterExpression:          expr.Filter(),
	})

	if err != nil {
		return nil, err
	}

	var items []*Forecast
	err = dynamodbattribute.UnmarshalListOfMaps(result.Items, &items)
	if err != nil {
		return nil, err
	}

	return items, nil
}

// SaveRecord to dynamodb
func SaveRecord(asset *Forecast) error {
	item := map[string]*dynamodb.AttributeValue{
		"id": &dynamodb.AttributeValue{
			S: aws.String(asset.ID),
		},
		"secondaryID": &dynamodb.AttributeValue{
			S: aws.String(asset.SecondaryID),
		},
		"location": &dynamodb.AttributeValue{
			S: aws.String(asset.Location),
		},
		"state": &dynamodb.AttributeValue{
			S: aws.String(asset.State),
		},
		"date": &dynamodb.AttributeValue{
			S: aws.String(asset.Date),
		},
	}

	// dynamodb doesn't allow to save empty string
	if len(asset.MinTemp) > 0 {
		item["minTemp"] = &dynamodb.AttributeValue{
			S: aws.String(asset.MinTemp),
		}
	}

	if len(asset.MaxTemp) > 0 {
		item["maxTemp"] = &dynamodb.AttributeValue{
			S: aws.String(asset.MaxTemp),
		}
	}

	if len(asset.Precis) > 0 {
		item["precis"] = &dynamodb.AttributeValue{
			S: aws.String(asset.Precis),
		}
	}

	if len(asset.Precipitation) > 0 {
		item["precipitation"] = &dynamodb.AttributeValue{
			S: aws.String(asset.Precipitation),
		}
	}

	_, err := db.PutItem(&dynamodb.PutItemInput{
		TableName: aws.String(tableName),
		Item:      item,
	})

	return err
}
