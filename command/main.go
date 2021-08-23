package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	log "github.com/sirupsen/logrus"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/secretsmanager"
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
)

type CommandOutput struct {
	Message string `json:"message"`
}

type CommandInput struct {
	Target    string `json:"target"`
	Db        string `json:"db"`
	Statement string `json:"Statement"`
}

type DbSecret struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Engine   string `json:"engine"`
	Host     string `json:"host"`
	Port     int64  `json:"port"`
}

func init() {
	log.SetLevel(log.DebugLevel)
}

func generateOutput(message string) (output string) {
	commandOutput := CommandOutput{
		Message: "command success",
	}

	outputByte, _ := json.Marshal(&commandOutput)
	output = string(outputByte)
	return
}

func getSecret(target, db string) (dbConn, dbEngine string) {
	log.Debugln("getSecret start")
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
	dbEngine = dbSecret.Engine
	if dbEngine == "mysql" {
		log.Debugln("engine mysql")
		dbConn = fmt.Sprintf("%s:%s@tcp(%s:%d)/%s",
			dbSecret.Username,
			dbSecret.Password,
			dbSecret.Host,
			dbSecret.Port,
			db)
	} else if dbEngine == "postgres" {
		log.Debugln("engine postgres")
		dbConn = fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
			dbSecret.Host,
			dbSecret.Port,
			dbSecret.Username,
			dbSecret.Password,
			db)
	}

	log.WithField("secret", dbSecret.Host).Debugln("getSecret complete")
	return
}

func executeStatement(dbConn, dbEngine, statement string) (err error) {
	log.Debugln("executeStatement start")
	db, err := sql.Open(dbEngine, dbConn)
	if err != nil {
		log.Errorln(err)
		return
	}
	_, errExec := db.Exec(statement)
	if errExec != nil {
		err = errExec
		log.Errorln(err)
		return
	}

	log.Debugln("executeStatement complete")
	return
}

func parseBody(body []byte) (command CommandInput, err error) {
	log.Debugln("parseBody start")
	type RawBody struct {
		Key    string `json:"key"`
		Fields struct {
			Customfield10156 struct {
				Value string `json:"value"`
			} `json:"customfield_10156"`
			Customfield10160 struct {
				Value string `json:"value"`
			} `json:"customfield_10160"`
			Customfield10157 string `json:"customfield_10157"`
			// Summary          string `json:"summary"`
		} `json:"fields"`
	}

	var raw RawBody

	err = json.Unmarshal([]byte(body), &raw)
	if err != nil {
		log.Errorln(err)
		return
	}

	command = CommandInput{
		Target:    raw.Fields.Customfield10156.Value,
		Db:        raw.Fields.Customfield10160.Value,
		Statement: raw.Fields.Customfield10157,
	}

	log.WithField("body", command.Target).Debugln("parseBody complete")
	return
}

func handler(request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	command, err := parseBody([]byte(request.Body))
	if err != nil {
		return events.APIGatewayProxyResponse{}, errors.New(err.Error())
	}

	dbConn, dbEngine := getSecret(command.Target, command.Db)
	err = executeStatement(dbConn, dbEngine, command.Statement)
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
