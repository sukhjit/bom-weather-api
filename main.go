package main

import (
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	ginadapter "github.com/awslabs/aws-lambda-go-api-proxy/gin"
	"github.com/gin-gonic/gin"
	"github.com/sukhjit/bom-weather-api/forecast"
	"github.com/sukhjit/bom-weather-api/util"
)

const (
	stdDateLayout     = "2006-01-02"
	compactDateLayout = "20060102"
)

var (
	errorLogger = log.New(os.Stderr, "[ERROR] ", log.Llongfile)
	ginLambda   *ginadapter.GinLambda
	isLambda    bool
)

func lambdaHandler(req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	if ginLambda == nil {
		r := newService()
		ginLambda = ginadapter.New(r)
	}

	return ginLambda.Proxy(req)
}

func webserver() {
	r := newService()
	r.Run(":8000")
}

func initEnv() {
	isLambda = false
	if web := os.Getenv("WEB"); web == "" {
		isLambda = true
	}
}

func main() {
	initEnv()

	// start lambda or webserver
	if isLambda {
		lambda.Start(lambdaHandler)
	} else {
		webserver()
	}
}

func newService() *gin.Engine {
	router := gin.Default()

	router.Use(corsMiddleware())
	router.GET("/status", statusHandler)
	router.GET("/weather/:location", weatherHandler)
	return router
}

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Content-Type", "application/json")
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, HEAD, PATCH, DELETE")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusOK)
			return
		}

		c.Next()
	}
}

func statusHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"info": "ok",
	})
}

func weatherHandler(c *gin.Context) {
	date := time.Now().Format(stdDateLayout)
	location := c.Param("location")
	state := ""

	// state provided
	exploded := strings.Split(location, ",")
	if len(exploded) == 2 {
		location = strings.Trim(exploded[0], " ")
		sn := strings.Trim(exploded[1], " ")
		if len(sn) > 0 {
			state = sn
		}
	}

	// date provided
	dateStr := c.DefaultQuery("date", "")
	if len(dateStr) == 8 {
		t, err := time.Parse(compactDateLayout, dateStr)
		if err != nil {
			clientError(c, http.StatusBadRequest, "date format is incorrect")
			return
		}

		date = t.Format(stdDateLayout)
	}

	items, err := forecast.GetItemsBySecondaryID(location, date)
	if err != nil {
		serverError(c, err.Error())
		return
	}

	if len(items) == 0 {
		clientError(c, http.StatusNotFound, "location not found")
		return
	}

	if len(items) > 1 {
		// found multiple locations, match state if given
		for _, row := range items {
			if row.State == state {
				c.JSON(http.StatusOK, row)
				return
			}
		}
	}

	c.JSON(http.StatusOK, items[0])
}

func clientError(c *gin.Context, code int, err string) {
	c.JSON(code, gin.H{
		"error": err,
	})
}

func serverError(c *gin.Context, err string) {
	errID := util.RandomString(8)

	errorLogger.Printf("ErrorID: %s, %v", errID, err)

	c.JSON(http.StatusInternalServerError, gin.H{
		"error": "Internal server error",
		"code":  errID,
	})
}
