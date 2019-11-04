package schema

import (
	"dynamic-graphql-api/handler/schema/graph"
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

type mutationField struct {
	fieldConfigCreate *graphql.InputObjectFieldConfig
	fieldConfigUpdate *graphql.InputObjectFieldConfig
	fieldConfigDelete *graphql.InputObjectFieldConfig
	column            string
}

func getMutationFields(g *graph.Graph, fields []*graph.Node) (map[string]mutationField, error) {
	mutationFields := map[string]mutationField{}

	for _, field := range fields {
		if !field.HasAttrKey("valueType") && field.GetAttrValueDefault("referenceType", "") != "forward" {
			continue
		}

		fieldName := field.GetAttrValueDefault("name", "")

		if field.GetAttrValueDefault("referenceType", "") == "forward" {
			referencedColumn := g.Edges().FilterSource(field).FilterEdgeType("fieldReferencesColumn").Targets().First()
			if referencedColumn == nil {
				return nil, errors.New("failed to find referenced column")
			}
			fieldName = strcase.ToLowerCamel(fieldName + "_" + referencedColumn.GetAttrValueDefault("name", ""))
		}

		column := g.Edges().FilterSource(field).FilterEdgeType("fieldHasColumn").Targets().First()
		if column == nil {
			return nil, errors.New("failed to find field's column")
		}

		fieldTypeCreate, fieldTypeUpdate, fieldTypeDelete, err := getMutationGraphqlTypeFromField(g, field)
		if err != nil {
			return nil, err
		}

		fieldDefinition := mutationField{column: column.GetAttrValueDefault("name", "")}
		if fieldTypeCreate != nil {
			fieldDefinition.fieldConfigCreate = &graphql.InputObjectFieldConfig{
				Type: fieldTypeCreate,
			}
		}
		if fieldTypeUpdate != nil {
			fieldDefinition.fieldConfigUpdate = &graphql.InputObjectFieldConfig{
				Type: fieldTypeUpdate,
			}
		}
		if fieldTypeDelete != nil {
			fieldDefinition.fieldConfigDelete = &graphql.InputObjectFieldConfig{
				Type: fieldTypeDelete,
			}
		}

		if fieldDefinition.fieldConfigCreate != nil || fieldDefinition.fieldConfigUpdate != nil || fieldDefinition.fieldConfigDelete != nil {
			mutationFields[fieldName] = fieldDefinition
		}
	}

	return nil, nil
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

		mutationFields, errTemp := getMutationFields(g, g.Edges().FilterSource(obj).FilterEdgeType("objectHasField").Targets().All())
		if err != nil {
			err = errTemp
			return false
		}

		for name, fieldDefinition := range mutationFields {
			if fieldDefinition.fieldConfigCreate != nil {
				inputFieldsCreate[name] = fieldDefinition.fieldConfigCreate
			}
			if fieldDefinition.fieldConfigUpdate != nil {
				inputFieldsUpdate[name] = fieldDefinition.fieldConfigUpdate
			}
			if fieldDefinition.fieldConfigDelete != nil {
				inputFieldsDelete[name] = fieldDefinition.fieldConfigDelete
			}
		}

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

		payloadClientMutationIDField := &graphql.Field{
			Type: graphql.NewNonNull(graphql.String),
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				payload, ok := p.Source.(mutationPayload)
				if !ok {
					return nil, errors.New("malformed source")
				}

				return payload.clientMutationID, nil
			},
		}
		payloadObjectField := &graphql.Field{
			Type: graphql.NewNonNull(graphqlObjects[objName]),
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				payload, ok := p.Source.(mutationPayload)
				if !ok {
					return nil, errors.New("malformed source")
				}

				return payload.c, nil
			},
		}

		payloadCreate := graphql.NewObject(graphql.ObjectConfig{
			Name: "Create" + objName + "Payload",
			Fields: graphql.Fields{
				"clientMutationId":            payloadClientMutationIDField,
				strcase.ToLowerCamel(objName): payloadObjectField,
			},
		})
		payloadUpdate := graphql.NewObject(graphql.ObjectConfig{
			Name: "Update" + objName + "Payload",
			Fields: graphql.Fields{
				"clientMutationId":            payloadClientMutationIDField,
				strcase.ToLowerCamel(objName): payloadObjectField,
			},
		})
		payloadDelete := graphql.NewObject(graphql.ObjectConfig{
			Name: "Delete" + objName + "Payload",
			Fields: graphql.Fields{
				"clientMutationId": payloadClientMutationIDField,
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
