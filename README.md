# Weather API #

This project leverages Golang, go-gin, aws sam, aws lambda, aws api gateway, and aws dynamodb

This code downloads the weather data from Bureau of Meteorology (BoM) http://www.bom.gov.au/catalogue/data-feeds.shtml, saves into dynamodb and then exposes that data using go-gin.

Each state of Australia has it's own xml file in Bom website. Inside the xml files each location of the given state has multiple forecast up to 7 days in the future. A lambda function is run at 2am to import the data from xml to dynamodb.

### API
API Gateway is used to deploy the front-end of the API, in the format of:
* ```/weather/sydney```
* ```/weather/sydney,nsw```

### Caveats:
* NO TESTS :(
* Due to massive writes, only NSW is imported into Dynamodb; rest have been commented out due to long write periods.
* Only current date data is saved into Dynamodb, the future date's data is ignored.
* To import all data, an SQS can be utilised to overcome the long write periods.
* PutItem function is used to save data to Dynamodb, with computed ID for each row, so that multiple writes for same location's data will update the data.

### Resources:
* https://github.com/nzoschke/gofaas
