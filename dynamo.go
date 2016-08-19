package main

import (
	"gopkg.in/underarmour/dynago.v1"

	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
)

var (
	AwsAccess   = os.Getenv("AWS_ACCESS_KEY")
	AwsSecret   = os.Getenv("AWS_SECRET_KEY")
	AwsRegion   = os.Getenv("AWS_REGION")
	DynamoTable = os.Getenv("DYNAMO_TABLE")
	ServiceKey  = os.Getenv("SERVICE_KEY")
	InstanceKey = os.Getenv("INSTANCE_KEY")
)

func main() {
	if AwsRegion == "" {
		AwsRegion = "us-west-2"
	}

	if DynamoTable == "" {
		DynamoTable = "service-config"
	}

	if InstanceKey == "" {
		InstanceKey = "id"
	}

	if ServiceKey == "" {
		ServiceKey = "service"
	}

	client := dynago.NewAwsClient(AwsRegion, AwsAccess, AwsSecret)

	args := os.Args[1:]
	if len(args) == 0 {
		fmt.Println(`
			put  service instance json
			get  service instance
			del  service instance
			list service [prefix]
		`)
	} else {
		if args[0] == "put" {
			if len(args) < 4 {
				log.Fatal("not enough args")
				return
			}

			service := args[1]
			instance := args[2]
			params := make(map[string]interface{})
			err := json.Unmarshal([]byte(args[3]), &params)
			if err != nil {
				log.Fatal(err)
			}

			err = put(client, service, instance, params)
			if err != nil {
				log.Fatal(err)
			}

			fmt.Println("OK")

		} else if args[0] == "get" {
			if len(args) < 3 {
				log.Fatal("not enough args")
				return
			}
			service := args[1]
			instance := args[2]

			val, err := get(client, service, instance)
			if err != nil {
				log.Fatal(err)
			}

			out(val)

		} else if args[0] == "list" {
			if len(args) < 2 {
				log.Fatal("not enough args")
				return
			}
			service := args[1]

			val, err := list(client, service)
			if err != nil {
				log.Fatal(err)
			}

			out(val)
		} else if args[0] == "del" {
			if len(args) < 3 {
				log.Fatal("not enough args")
				return
			}
			service := args[1]
			instance := args[2]

			err := del(client, service, instance)
			if err != nil {
				log.Fatal(err)
			}
			fmt.Println("OK")

		} else {
			log.Fatal("no command recognized")
		}
	}
}

func get(client *dynago.Client, service string, id string) (map[string]interface{}, error) {
	res, err := client.GetItem(DynamoTable, dynago.Document{ServiceKey: service, InstanceKey: id}).Execute()
	if err != nil {
		return nil, err
	}

	return parse(res.Item), nil
}

func list(client *dynago.Client, service string, prefixes ...string) ([]map[string]interface{}, error) {
	prefix := strings.Join(prefixes, "/")

	q := client.Query(DynamoTable).
		Param(":service", service).
		ConsistentRead(true)

	if prefix == "" {
		q = q.KeyConditionExpression(ServiceKey+" = :service")
	} else {
		qu := fmt.Sprintf("%s = :service AND begins_with(%s, :prefix)", ServiceKey, InstanceKey)
		q = q.KeyConditionExpression(qu).Param(":prefix", prefix)
	}

	res, err := q.Execute()
	if err != nil {
		return nil, err
	}

	results := make([]map[string]interface{}, 0, len(res.Items))
	for _, doc := range res.Items {
		results = append(results, parse(doc))
	}

	return results, nil
}

func del(client *dynago.Client, service, id string) error {
	_, err := client.DeleteItem(DynamoTable, dynago.Document{
		ServiceKey: service,
		InstanceKey: id,
	}).Execute()
	return err
}

func put(client *dynago.Client, service string, id string, params map[string]interface{}) error {
	params[ServiceKey] = service
	params[InstanceKey] = id
	_, err := client.PutItem(DynamoTable, params).Execute()
	return err
}

func parse(doc dynago.Document) map[string]interface{} {
	m := make(map[string]interface{})
	for _, p := range doc.AsParams() {
		m[p.Key] = p.Value
	}
	return m
}

func out(item interface{}) {
	data, err := json.MarshalIndent(item, " ", "  ")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("%s\n", data)
}
