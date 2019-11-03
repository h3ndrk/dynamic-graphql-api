package schema

import (
	"dynamic-graphql-api/handler/schema/graph"
	"fmt"
	"strings"

	"github.com/graphql-go/graphql"
	"github.com/iancoleman/strcase"
	"github.com/jinzhu/inflection"
	"github.com/pkg/errors"
)

var mutation *graphql.Object

type mutationPayload struct {
	clientMutationID string
	c                cursor
}

func getMutationGraphqlTypeFromField(g *graph.Graph, field *graph.Node) (graphql.Output, graphql.Output, graphql.Output, error) {
	//           | create         | update         | delete
	// ----------+----------------+----------------+---------
	// Int       | Int            | Int            | omit
	// Int!      | Int!           | Int            | omit
	// Float     | Float          | Float          | omit
	// Float!    | Float!         | Float          | omit
	// String    | String         | String         | omit
	// String!   | String!        | String         | omit
	// Boolean   | Boolean        | Boolean        | omit
	// Boolean!  | Boolean!       | Boolean        | omit
	// ID        | does not exist | does not exist | omit
	// ID!       | omit           | ID!            | required
	// DateTime  | DateTime       | DateTime       | omit
	// DateTime! | DateTime!      | DateTime       | omit
	// forward   | forward        | forward        | omit
	// forward!  | forward!       | forward        | omit
	// backward  | omit           | omit           | omit
	// joined    | separate       | separate       | omit

	var (
		createType graphql.Output
		updateType graphql.Output
		deleteType graphql.Output
	)
	if field.HasAttrKey("valueType") {
		valueType := field.GetAttrValueDefault("valueType", "")
		valueTypeWithoutNonNull := strings.TrimSuffix(valueType, "!")
		isNonNull := valueType != valueTypeWithoutNonNull

		switch valueTypeWithoutNonNull {
		case "Int":
			createType = graphql.Int
			updateType = graphql.Int
		case "Float":
			createType = graphql.Float
			updateType = graphql.Float
		case "String":
			createType = graphql.String
			updateType = graphql.String
		case "Boolean":
			createType = graphql.Boolean
			updateType = graphql.Boolean
		case "ID":
			updateType = graphql.NewNonNull(graphql.ID)
			deleteType = graphql.NewNonNull(graphql.ID)
		case "DateTime":
			createType = graphql.DateTime
			updateType = graphql.DateTime
		default:
			return nil, nil, nil, errors.Errorf("unsupported type %s", valueTypeWithoutNonNull)
		}

		if valueTypeWithoutNonNull != "ID" && isNonNull {
			createType = graphql.NewNonNull(createType)
		}
	} else if field.HasAttrKey("referenceType") {
		if field.GetAttrValueDefault("referenceType", "") == "forward" {
			createType = graphql.ID
			updateType = graphql.ID
		}

		if field.GetAttrValueDefault("isNonNull", "false") == "true" {
			createType = graphql.NewNonNull(createType)
		}
	}

	return createType, updateType, deleteType, nil
}

func initMutation(g *graph.Graph) error {
	mutation = graphql.NewObject(graphql.ObjectConfig{
		Name:   "Mutation",
		Fields: graphql.Fields{},
	})

	var err error
	g.Nodes().FilterObjects().ForEach(func(obj *graph.Node) bool {
		objName := obj.GetAttrValueDefault("name", "")

		inputFieldsCreate := graphql.InputObjectConfigFieldMap{}
		inputFieldsUpdate := graphql.InputObjectConfigFieldMap{}
		inputFieldsDelete := graphql.InputObjectConfigFieldMap{}

		g.Edges().FilterSource(obj).FilterEdgeType("objectHasField").Targets().ForEach(func(field *graph.Node) bool {
			fieldName := field.GetAttrValueDefault("name", "")

			fieldTypeCreate, fieldTypeUpdate, fieldTypeDelete, errTemp := getMutationGraphqlTypeFromField(g, field)
			if errTemp != nil {
				err = errTemp
				return false
			}

			fmt.Printf("%s.%s: %+v %+v %+v\n", objName, fieldName, fieldTypeCreate, fieldTypeUpdate, fieldTypeDelete)

			if fieldTypeCreate != nil {
				inputFieldsCreate[fieldName] = &graphql.InputObjectFieldConfig{
					Type: fieldTypeCreate,
				}
			}
			if fieldTypeUpdate != nil {
				inputFieldsUpdate[fieldName] = &graphql.InputObjectFieldConfig{
					Type: fieldTypeUpdate,
				}
			}
			if fieldTypeDelete != nil {
				inputFieldsDelete[fieldName] = &graphql.InputObjectFieldConfig{
					Type: fieldTypeDelete,
				}
			}

			return true
		})

		inputFieldsCreate["clientMutationId"] = &graphql.InputObjectFieldConfig{
			Type: graphql.NewNonNull(graphql.String),
		}
		inputFieldsUpdate["clientMutationId"] = &graphql.InputObjectFieldConfig{
			Type: graphql.NewNonNull(graphql.String),
		}
		inputFieldsDelete["clientMutationId"] = &graphql.InputObjectFieldConfig{
			Type: graphql.NewNonNull(graphql.String),
		}

		inputCreate := graphql.NewInputObject(graphql.InputObjectConfig{
			Name:   "Create" + objName + "Input",
			Fields: inputFieldsCreate,
		})
		inputUpdate := graphql.NewInputObject(graphql.InputObjectConfig{
			Name:   "Update" + objName + "Input",
			Fields: inputFieldsUpdate,
		})
		inputDelete := graphql.NewInputObject(graphql.InputObjectConfig{
			Name:   "Delete" + objName + "Input",
			Fields: inputFieldsDelete,
		})

		payloadCreate := graphql.NewObject(graphql.ObjectConfig{
			Name: "Create" + objName + "Payload",
			Fields: graphql.Fields{
				"clientMutationId": &graphql.Field{
					Type: graphql.NewNonNull(graphql.String),
					Resolve: func(p graphql.ResolveParams) (interface{}, error) {
						payload, ok := p.Source.(mutationPayload)
						if !ok {
							return nil, errors.New("malformed source")
						}

						return payload.clientMutationID, nil
					},
				},
				strcase.ToLowerCamel(objName): &graphql.Field{
					Type: graphql.NewNonNull(graphqlObjects[objName]),
					Resolve: func(p graphql.ResolveParams) (interface{}, error) {
						payload, ok := p.Source.(mutationPayload)
						if !ok {
							return nil, errors.New("malformed source")
						}

						return payload.c, nil
					},
				},
			},
		})
		payloadUpdate := graphql.NewObject(graphql.ObjectConfig{
			Name: "Update" + objName + "Payload",
			Fields: graphql.Fields{
				"clientMutationId": &graphql.Field{
					Type: graphql.NewNonNull(graphql.String),
					Resolve: func(p graphql.ResolveParams) (interface{}, error) {
						payload, ok := p.Source.(mutationPayload)
						if !ok {
							return nil, errors.New("malformed source")
						}

						return payload.clientMutationID, nil
					},
				},
				strcase.ToLowerCamel(objName): &graphql.Field{
					Type: graphql.NewNonNull(graphqlObjects[objName]),
					Resolve: func(p graphql.ResolveParams) (interface{}, error) {
						payload, ok := p.Source.(mutationPayload)
						if !ok {
							return nil, errors.New("malformed source")
						}

						return payload.c, nil
					},
				},
			},
		})
		payloadDelete := graphql.NewObject(graphql.ObjectConfig{
			Name: "Delete" + objName + "Payload",
			Fields: graphql.Fields{
				"clientMutationId": &graphql.Field{
					Type: graphql.NewNonNull(graphql.String),
					Resolve: func(p graphql.ResolveParams) (interface{}, error) {
						payload, ok := p.Source.(mutationPayload)
						if !ok {
							return nil, errors.New("malformed source")
						}

						return payload.clientMutationID, nil
					},
				},
			},
		})

		referencedTable := g.Edges().FilterSource(obj).FilterEdgeType("objectHasTable").Targets().First()
		if referencedTable == nil {
			err = errors.New("referenced table not found")
			return false
		}

		mutation.AddFieldConfig(inflection.Singular(strcase.ToLowerCamel("create_"+objName)), &graphql.Field{
			Type: graphql.NewNonNull(payloadCreate),
			Args: graphql.FieldConfigArgument{
				"input": &graphql.ArgumentConfig{
					Type: graphql.NewNonNull(inputCreate),
				},
			},
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				return nil, nil
			},
		})
		mutation.AddFieldConfig(inflection.Singular(strcase.ToLowerCamel("update_"+objName)), &graphql.Field{
			Type: graphql.NewNonNull(payloadUpdate),
			Args: graphql.FieldConfigArgument{
				"input": &graphql.ArgumentConfig{
					Type: graphql.NewNonNull(inputUpdate),
				},
			},
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				return nil, nil
			},
		})
		mutation.AddFieldConfig(inflection.Singular(strcase.ToLowerCamel("delete_"+objName)), &graphql.Field{
			Type: graphql.NewNonNull(payloadDelete),
			Args: graphql.FieldConfigArgument{
				"input": &graphql.ArgumentConfig{
					Type: graphql.NewNonNull(inputDelete),
				},
			},
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				return nil, nil
			},
		})

		return true
	})

	return err
}
