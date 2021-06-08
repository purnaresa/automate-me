package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/secretsmanager"
	_ "github.com/go-sql-driver/mysql"
)

type CommandOutput struct {
	Message string `json:"message"`
}

type CommandInput struct {
	Target    string `json:"target"`
	Db        string `json:"db"`
	Statement string `json:"Statement"`
}

func generateOutput(message string) (output string) {
	commandOutput := CommandOutput{
		Message: "command success",
	}

	outputByte, _ := json.Marshal(&commandOutput)
	output = string(outputByte)
	return
}

type DbSecret struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Engine   string `json:"engine\"`
	Host     string `json:"host"`
	Port     int64  `json:"port"`
}

func getSecret(target, db string) (dbConn, dbEngine string) {
	svc := secretsmanager.New(session.New())
	input := &secretsmanager.GetSecretValueInput{
		SecretId: aws.String(target),
	}

	result, err := svc.GetSecretValue(input)
	if err != nil {
		log.Println(err)
		return
	}
	dbSecret := DbSecret{}
	errUnmarshal := json.Unmarshal([]byte(*result.SecretString), &dbSecret)
	if errUnmarshal != nil {
		log.Println(err)
	}
	dbConn = fmt.Sprintf("%s:%s@tcp(%s:%d)/%s",
		dbSecret.Username,
		dbSecret.Password,
		dbSecret.Host,
		dbSecret.Port,
		db)
	dbEngine = dbSecret.Engine
	return
}

func executeStatement(dbConn, dbEngine, statement string) (err error) {
	db, err := sql.Open(dbEngine, dbConn)
	if err != nil {
		log.Println(err)
	}
	_, errExec := db.Exec(statement)
	if errExec != nil {
		err = errExec
		log.Println(err)
	}
	return
}

func handler(request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	var command CommandInput
	jsonErr := json.Unmarshal([]byte(request.Body), &command)
	if jsonErr != nil {
		return events.APIGatewayProxyResponse{}, errors.New(jsonErr.Error())
	}

	dbConn, dbEngine := getSecret(command.Target, command.Db)
	log.Println(dbConn)
	log.Println(dbEngine)
	err := executeStatement(dbConn, dbEngine, command.Statement)
	if err != nil {
		return events.APIGatewayProxyResponse{}, err
	}

	return events.APIGatewayProxyResponse{
		Body:       generateOutput("Command successfull"),
		StatusCode: 200,
	}, nil
}

func main() {
	lambda.Start(handler)
}
