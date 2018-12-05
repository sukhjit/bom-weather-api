export AWS_DEFAULT_REGION ?= ap-southeast-2
APP ?= bom-weather-api
BUCKET = pkgs-$(shell aws sts get-caller-identity --output text --query 'Account')-$(AWS_DEFAULT_REGION)
DEPLOY_BUCKET = deploy-$(shell aws sts get-caller-identity --output text --query 'Account')-$(AWS_DEFAULT_REGION)

clean:
	rm -f main out.yml

deploy: PARAMS ?= =
deploy:
	@aws s3api head-bucket --bucket $(BUCKET) || aws s3 mb s3://$(BUCKET) --region $(AWS_DEFAULT_REGION)
	@aws s3api head-bucket --bucket $(DEPLOY_BUCKET) || aws s3 mb s3://$(DEPLOY_BUCKET) --region $(AWS_DEFAULT_REGION)
	sam package --output-template-file out.yml --s3-bucket $(BUCKET) --template-file scripts/aws-template.yaml
	sam deploy --capabilities CAPABILITY_NAMED_IAM --parameter-overrides $(PARAMS) --template-file out.yml --stack-name $(APP)

api: ZIP_FILE_NAME = $(APP)-bwa-main.zip
api: FUNCTION_NAME=$(shell aws cloudformation describe-stacks --output text --query 'Stacks[].Outputs[?OutputKey==`ApiFunction`].{Value:OutputValue}' --stack-name $(APP))
api:
	GOOS=linux go build -o main main.go
	zip -j /tmp/$(ZIP_FILE_NAME) main
	aws s3 cp /tmp/$(ZIP_FILE_NAME) s3://$(DEPLOY_BUCKET)/$(ZIP_FILE_NAME)
	aws lambda update-function-code \
        --function-name $(FUNCTION_NAME) \
        --s3-bucket $(DEPLOY_BUCKET) \
        --s3-key $(ZIP_FILE_NAME)

func-pw: ZIP_FILE_NAME = $(APP)-populate-weather.zip
func-pw: FUNCTION_NAME=$(shell aws cloudformation describe-stacks --output text --query 'Stacks[].Outputs[?OutputKey==`PopulateWeatherFunction`].{Value:OutputValue}' --stack-name $(APP))
func-pw:
	GOOS=linux go build -o main alone-func/populate-weather/main.go
	zip -j /tmp/$(ZIP_FILE_NAME) main
	aws s3 cp /tmp/$(ZIP_FILE_NAME) s3://$(DEPLOY_BUCKET)/$(ZIP_FILE_NAME)
	aws lambda update-function-code \
        --function-name $(FUNCTION_NAME) \
        --s3-bucket $(DEPLOY_BUCKET) \
        --s3-key $(ZIP_FILE_NAME)
