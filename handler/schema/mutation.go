package schema

import (
	"dynamic-graphql-api/handler/schema/db"
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
	referencedC      cursor
}

func getMutationGraphqlTypeFromField(g *graph.Graph, field *graph.Node) (graphql.Output, graphql.Output, graphql.Output, string, error) {
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
		createType           graphql.Output
		updateType           graphql.Output
		deleteType           graphql.Output
		referencedObjectName string
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
			return nil, nil, nil, "", errors.Errorf("unsupported type %s", valueTypeWithoutNonNull)
		}

		if valueTypeWithoutNonNull != "ID" && isNonNull {
			createType = graphql.NewNonNull(createType)
		}
	} else if field.HasAttrKey("referenceType") {
		referencedObject := g.Edges().FilterSource(field).FilterEdgeType("fieldReferencesObject").Targets().First()
		if referencedObject == nil {
			return nil, nil, nil, "", errors.Errorf("field %+v does not reference object", field.Attrs)
		}
		referencedObjectName = referencedObject.GetAttrValueDefault("name", "")

		if field.GetAttrValueDefault("referenceType", "") == "forward" {
			createType = graphql.ID
			updateType = graphql.ID
		}

		if field.GetAttrValueDefault("isNonNull", "false") == "true" {
			createType = graphql.NewNonNull(createType)
		}
	}

	return createType, updateType, deleteType, referencedObjectName, nil
}

type mutationField struct {
	fieldConfigCreate    *graphql.InputObjectFieldConfig
	fieldConfigUpdate    *graphql.InputObjectFieldConfig
	fieldConfigDelete    *graphql.InputObjectFieldConfig
	column               string
	isPrimaryKey         bool
	referencedObjectName string
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

		fieldTypeCreate, fieldTypeUpdate, fieldTypeDelete, referencedObjectName, err := getMutationGraphqlTypeFromField(g, field)
		if err != nil {
			return nil, err
		}

		fieldDefinition := mutationField{
			column:               column.GetAttrValueDefault("name", ""),
			isPrimaryKey:         column.GetAttrValueDefault("isPrimaryKey", "false") == "true",
			referencedObjectName: referencedObjectName,
		}
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

	return mutationFields, nil
}

func addMutationAssociations(g *graph.Graph, obj *graph.Node) error {
	objName := obj.GetAttrValueDefault("name", "")

	// iterate over fields, filter joined references
	var err error
	g.Edges().FilterSource(obj).FilterEdgeType("objectHasField").Targets().ForEach(func(field *graph.Node) bool {
		fieldName := field.GetAttrValueDefault("name", "")

		if field.GetAttrValueDefault("referenceType", "") != "joined" {
			return true
		}

		referencedObject := g.Edges().FilterSource(field).FilterEdgeType("fieldReferencesObject").Targets().First()
		if referencedObject == nil {
			err = errors.Errorf("missing reference to object from joined field %s.%s", objName, fieldName)
			return false
		}
		referencedObjectName := referencedObject.GetAttrValueDefault("name", "")

		joinTable := g.Edges().FilterSource(field).FilterEdgeType("fieldReferencesJoinTable").Targets().First()
		if joinTable == nil {
			err = errors.New("join table not found")
			return false
		}
		ownColumn := g.Edges().FilterSource(field).FilterEdgeType("fieldReferencesOwnJoinColumn").Targets().First()
		if ownColumn == nil {
			err = errors.New("own column not found")
			return false
		}
		foreignColumn := g.Edges().FilterSource(field).FilterEdgeType("fieldReferencesForeignJoinColumn").Targets().First()
		if foreignColumn == nil {
			err = errors.New("foreign column not found")
			return false
		}

		fmt.Printf("Found field with joined mutation: %s.%s -> %s\n", objName, fieldName, referencedObjectName)

		associationName := inflection.Singular(objName) + "_" + inflection.Singular(referencedObjectName)
		if objName > referencedObjectName {
			// only create mutation where objName and referenceObjectName are alphabetically ordered
			// so that we can use the names later
			return true
		}

		input := graphql.NewInputObject(graphql.InputObjectConfig{
			Name: strcase.ToCamel("association_" + associationName + "_input"),
			Fields: graphql.InputObjectConfigFieldMap{
				"clientMutationId": &graphql.InputObjectFieldConfig{
					Type: graphql.NewNonNull(graphql.String),
				},
				strcase.ToLowerCamel(objName + "_id"): &graphql.InputObjectFieldConfig{
					Type: graphql.NewNonNull(graphql.ID),
				},
				strcase.ToLowerCamel(referencedObjectName + "_id"): &graphql.InputObjectFieldConfig{
					Type: graphql.NewNonNull(graphql.ID),
				},
			},
		})

		payload := graphql.NewObject(graphql.ObjectConfig{
			Name: strcase.ToCamel("association_" + associationName + "_payload"),
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
				strcase.ToLowerCamel(referencedObjectName): &graphql.Field{
					Type: graphql.NewNonNull(graphqlObjects[referencedObjectName]),
					Resolve: func(p graphql.ResolveParams) (interface{}, error) {
						payload, ok := p.Source.(mutationPayload)
						if !ok {
							return nil, errors.New("malformed source")
						}

						return payload.referencedC, nil
					},
				},
			},
		})

		mutation.AddFieldConfig(strcase.ToLowerCamel("associate_"+associationName), &graphql.Field{
			Type: graphql.NewNonNull(payload),
			Args: graphql.FieldConfigArgument{
				"input": &graphql.ArgumentConfig{
					Type: graphql.NewNonNull(input),
				},
			},
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				inputInterface, ok := p.Args["input"]
				if !ok {
					return nil, errors.New("missing input")
				}
				input, ok := inputInterface.(map[string]interface{})
				if !ok {
					return nil, errors.New("malformed input")
				}

				dbFromContext, err := getDBFromContext(p.Context)
				if err != nil {
					return nil, err
				}

				var (
					objID              uint
					referencedObjectID uint
				)
				if inputField, ok := input[strcase.ToLowerCamel(objName+"_id")]; ok {
					if inputField, ok := inputField.(string); ok {
						c, err := parseCursor(inputField)
						if err != nil {
							return nil, err
						}
						if c.object != objName {
							return nil, errors.Errorf("unexpected id type %s of field (expected %s)", c.object, objName)
						}

						objID = c.id
					}
				}
				if inputField, ok := input[strcase.ToLowerCamel(referencedObjectName+"_id")]; ok {
					if inputField, ok := inputField.(string); ok {
						c, err := parseCursor(inputField)
						if err != nil {
							return nil, err
						}
						if c.object != referencedObjectName {
							return nil, errors.Errorf("unexpected id type %s of field (expected %s)", c.object, objName)
						}

						referencedObjectID = c.id
					}
				}

				err = db.MutationAssociateQuery(db.MutationAssociateRequest{
					Ctx: p.Context,
					DB:  dbFromContext,

					Table: joinTable.GetAttrValueDefault("name", ""),
					ColumnValues: map[string]interface{}{
						ownColumn.GetAttrValueDefault("name", ""):     objID,
						foreignColumn.GetAttrValueDefault("name", ""): referencedObjectID,
					},
				})
				if err != nil {
					return nil, err
				}

				var payload mutationPayload
				payload.c = cursor{object: objName, id: objID}
				payload.referencedC = cursor{object: referencedObjectName, id: referencedObjectID}

				if clientMutationID, ok := input["clientMutationId"]; ok {
					if clientMutationID, ok := clientMutationID.(string); ok {
						payload.clientMutationID = clientMutationID
					}
				}

				return payload, nil
			},
		})
		mutation.AddFieldConfig(strcase.ToLowerCamel("disassociate_"+associationName), &graphql.Field{
			Type: graphql.NewNonNull(payload),
			Args: graphql.FieldConfigArgument{
				"input": &graphql.ArgumentConfig{
					Type: graphql.NewNonNull(input),
				},
			},
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				inputInterface, ok := p.Args["input"]
				if !ok {
					return nil, errors.New("missing input")
				}
				input, ok := inputInterface.(map[string]interface{})
				if !ok {
					return nil, errors.New("malformed input")
				}

				dbFromContext, err := getDBFromContext(p.Context)
				if err != nil {
					return nil, err
				}

				var (
					objID              uint
					referencedObjectID uint
				)
				if inputField, ok := input[strcase.ToLowerCamel(objName+"_id")]; ok {
					if inputField, ok := inputField.(string); ok {
						c, err := parseCursor(inputField)
						if err != nil {
							return nil, err
						}
						if c.object != objName {
							return nil, errors.Errorf("unexpected id type %s of field (expected %s)", c.object, objName)
						}

						objID = c.id
					}
				}
				if inputField, ok := input[strcase.ToLowerCamel(referencedObjectName+"_id")]; ok {
					if inputField, ok := inputField.(string); ok {
						c, err := parseCursor(inputField)
						if err != nil {
							return nil, err
						}
						if c.object != referencedObjectName {
							return nil, errors.Errorf("unexpected id type %s of field (expected %s)", c.object, objName)
						}

						referencedObjectID = c.id
					}
				}

				err = db.MutationDisassociateQuery(db.MutationDisassociateRequest{
					Ctx: p.Context,
					DB:  dbFromContext,

					Table: joinTable.GetAttrValueDefault("name", ""),
					ColumnValues: map[string]interface{}{
						ownColumn.GetAttrValueDefault("name", ""):     objID,
						foreignColumn.GetAttrValueDefault("name", ""): referencedObjectID,
					},
				})
				if err != nil {
					return nil, err
				}

				var payload mutationPayload
				payload.c = cursor{object: objName, id: objID}
				payload.referencedC = cursor{object: referencedObjectName, id: referencedObjectID}

				if clientMutationID, ok := input["clientMutationId"]; ok {
					if clientMutationID, ok := clientMutationID.(string); ok {
						payload.clientMutationID = clientMutationID
					}
				}

				return payload, nil
			},
		})

		return true
	})

	return err
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
				inputInterface, ok := p.Args["input"]
				if !ok {
					return nil, errors.New("missing input")
				}
				input, ok := inputInterface.(map[string]interface{})
				if !ok {
					return nil, errors.New("malformed input")
				}

				dbFromContext, err := getDBFromContext(p.Context)
				if err != nil {
					return nil, err
				}

				// check inputs availability (required & defined)
				columns := map[string]interface{}{}
				for name, inputField := range input {
					if name == "clientMutationId" {
						continue
					}

					if fieldDefinition, ok := mutationFields[name]; ok && fieldDefinition.fieldConfigCreate != nil {
						valueType := fieldDefinition.fieldConfigCreate.Type.Name()
						valueTypeWithoutNonNull := strings.TrimSuffix(valueType, "!")
						if valueTypeWithoutNonNull == "ID" {
							inputFieldString, ok := inputField.(string)
							if !ok {
								return nil, errors.Errorf("unknown id type of field %s", name)
							}
							c, err := parseCursor(inputFieldString)
							if err != nil {
								return nil, err
							}
							if fieldDefinition.isPrimaryKey && c.object != objName {
								return nil, errors.Errorf("unexpected id type %s of field %s (expected %s)", c.object, name, objName)
							}
							if !fieldDefinition.isPrimaryKey && c.object != fieldDefinition.referencedObjectName {
								return nil, errors.Errorf("unexpected id type %s of field %s (expected %s)", c.object, name, fieldDefinition.referencedObjectName)
							}

							columns[fieldDefinition.column] = c.id
						} else {
							columns[fieldDefinition.column] = inputField
						}
					} else {
						return nil, errors.Errorf("unexpected input field %s", name)
					}
				}
				for name, fieldDefinition := range mutationFields {
					if fieldDefinition.fieldConfigCreate == nil {
						continue
					}

					if _, ok := fieldDefinition.fieldConfigCreate.Type.(*graphql.NonNull); ok {
						if _, ok := input[name]; !ok {
							return nil, errors.Errorf("missing required input field %s", name)
						}
					}
				}

				insertedID, err := db.MutationCreateQuery(db.MutationCreateRequest{
					Ctx: p.Context,
					DB:  dbFromContext,

					Table:        referencedTable.GetAttrValueDefault("name", ""),
					ColumnValues: columns,
				})
				if err != nil {
					return nil, err
				}

				var payload mutationPayload
				payload.c = cursor{object: objName, id: insertedID}

				if clientMutationID, ok := input["clientMutationId"]; ok {
					if clientMutationID, ok := clientMutationID.(string); ok {
						payload.clientMutationID = clientMutationID
					}
				}

				return payload, nil
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
				inputInterface, ok := p.Args["input"]
				if !ok {
					return nil, errors.New("missing input")
				}
				input, ok := inputInterface.(map[string]interface{})
				if !ok {
					return nil, errors.New("malformed input")
				}

				dbFromContext, err := getDBFromContext(p.Context)
				if err != nil {
					return nil, err
				}

				// check inputs availability (required & defined)
				var columnWithPrimaryKey string
				columns := map[string]interface{}{}
				for name, inputField := range input {
					if name == "clientMutationId" {
						continue
					}

					if fieldDefinition, ok := mutationFields[name]; ok && fieldDefinition.fieldConfigUpdate != nil {
						valueType := fieldDefinition.fieldConfigUpdate.Type.Name()
						valueTypeWithoutNonNull := strings.TrimSuffix(valueType, "!")
						if valueTypeWithoutNonNull == "ID" {
							inputFieldString, ok := inputField.(string)
							if !ok {
								return nil, errors.Errorf("unknown id type of field %s", name)
							}
							c, err := parseCursor(inputFieldString)
							if err != nil {
								return nil, err
							}
							if fieldDefinition.isPrimaryKey && c.object != objName {
								return nil, errors.Errorf("unexpected id type %s of field %s (expected %s)", c.object, name, objName)
							}
							if !fieldDefinition.isPrimaryKey && c.object != fieldDefinition.referencedObjectName {
								return nil, errors.Errorf("unexpected id type %s of field %s (expected %s)", c.object, name, fieldDefinition.referencedObjectName)
							}

							columns[fieldDefinition.column] = c.id
						} else {
							columns[fieldDefinition.column] = inputField
						}
						if fieldDefinition.isPrimaryKey {
							columnWithPrimaryKey = name
						}
					} else {
						return nil, errors.Errorf("unexpected input field %s", name)
					}
				}
				for name, fieldDefinition := range mutationFields {
					if fieldDefinition.fieldConfigUpdate == nil {
						continue
					}

					if _, ok := fieldDefinition.fieldConfigUpdate.Type.(*graphql.NonNull); ok {
						if _, ok := input[name]; !ok {
							return nil, errors.Errorf("missing required input field %s", name)
						}
					}
				}
				if columnWithPrimaryKey == "" {
					return nil, errors.New("missing identification field")
				}

				err = db.MutationUpdateQuery(db.MutationUpdateRequest{
					Ctx: p.Context,
					DB:  dbFromContext,

					Table:                referencedTable.GetAttrValueDefault("name", ""),
					ColumnValues:         columns,
					ColumnWithPrimaryKey: columnWithPrimaryKey,
				})
				if err != nil {
					return nil, err
				}

				var payload mutationPayload
				payload.c = cursor{object: objName, id: columns[columnWithPrimaryKey].(uint)}

				if clientMutationID, ok := input["clientMutationId"]; ok {
					if clientMutationID, ok := clientMutationID.(string); ok {
						payload.clientMutationID = clientMutationID
					}
				}

				return payload, nil
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
				inputInterface, ok := p.Args["input"]
				if !ok {
					return nil, errors.New("missing input")
				}
				input, ok := inputInterface.(map[string]interface{})
				if !ok {
					return nil, errors.New("malformed input")
				}

				dbFromContext, err := getDBFromContext(p.Context)
				if err != nil {
					return nil, err
				}

				// check inputs availability (required & defined)
				var (
					columnName  string
					columnValue interface{}
				)
				for name, inputField := range input {
					if name == "clientMutationId" {
						continue
					}

					if fieldDefinition, ok := mutationFields[name]; ok && fieldDefinition.fieldConfigDelete != nil {
						valueType := fieldDefinition.fieldConfigDelete.Type.Name()
						valueTypeWithoutNonNull := strings.TrimSuffix(valueType, "!")
						if valueTypeWithoutNonNull == "ID" && fieldDefinition.isPrimaryKey {
							inputFieldString, ok := inputField.(string)
							if !ok {
								return nil, errors.Errorf("unknown id type of field %s", name)
							}
							c, err := parseCursor(inputFieldString)
							if err != nil {
								return nil, err
							}
							if c.object != objName {
								return nil, errors.Errorf("unexpected id type %s of field %s (expected %s)", c.object, name, objName)
							}

							columnName = fieldDefinition.column
							columnValue = c.id
						} // else: ignore other fields
					} else {
						return nil, errors.Errorf("unexpected input field %s", name)
					}
				}
				for name, fieldDefinition := range mutationFields {
					if fieldDefinition.fieldConfigDelete == nil {
						continue
					}

					if _, ok := fieldDefinition.fieldConfigDelete.Type.(*graphql.NonNull); ok {
						if _, ok := input[name]; !ok {
							return nil, errors.Errorf("missing required input field %s", name)
						}
					}
				}
				if columnName == "" {
					return nil, errors.New("missing identification field")
				}

				err = db.MutationDeleteQuery(db.MutationDeleteRequest{
					Ctx: p.Context,
					DB:  dbFromContext,

					Table:       referencedTable.GetAttrValueDefault("name", ""),
					ColumnName:  columnName,
					ColumnValue: columnValue,
				})
				if err != nil {
					return nil, err
				}

				var payload mutationPayload
				// skipping payload.c because Delete*Payload ignores it

				if clientMutationID, ok := input["clientMutationId"]; ok {
					if clientMutationID, ok := clientMutationID.(string); ok {
						payload.clientMutationID = clientMutationID
					}
				}

				return payload, nil
			},
		})

		err = addMutationAssociations(g, obj)
		if err != nil {
			return false
		}

		return true
	})

	fmt.Printf("Mutations:\n")
	for name := range mutation.Fields() {
		fmt.Printf("  %s\n", name)
	}

	return err
}
