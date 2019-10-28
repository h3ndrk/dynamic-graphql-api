package associations

import (
	"fmt"
	"strings"

	parse "github.com/h3ndrk/go-sqlite-createtable-parser"
	"github.com/pkg/errors"
)

// type reference struct {
// 	sourceColumn string
// 	targetTable  string
// 	targetColumn string
// }

// func getReferencesFromStmt(stmt *parse.Table) []reference {
// 	var references []reference

// 	for _, column := range stmt.Columns {
// 		if column.Name != nil && column.ForeignKey != nil && column.ForeignKey.Table != nil && len(column.ForeignKey.Columns) == 1 {
// 			references = append(references, reference{
// 				sourceColumn: *column.Name,
// 				targetTable:  *column.ForeignKey.Table,
// 				targetColumn: column.ForeignKey.Columns[0],
// 			})
// 		}
// 	}

// 	for _, constraint := range stmt.TableConstraints {
// 		if constraint.Type == parse.TableConstraintTypeForeignKey && len(constraint.ForeignKeyColumns) == 1 && constraint.ForeignKey != nil && constraint.ForeignKey.Table != nil && len(constraint.ForeignKey.Columns) == 1 {
// 			references = append(references, reference{
// 				sourceColumn: constraint.ForeignKeyColumns[0],
// 				targetTable:  *constraint.ForeignKey.Table,
// 				targetColumn: constraint.ForeignKey.Columns[0],
// 			})
// 		}
// 	}

// 	return references
// }

// type column struct {
// 	name         string
// 	typeAffinity string
// 	notNull      bool
// }

// func getColumnsFromStmt(stmt *parse.Table) []column {
// 	var columns []column

// 	for _, c := range stmt.Columns {
// 		if c.Name != nil {
// 			var typeAffinity string
// 			if c.Type != nil {
// 				typeAffinity = *c.Type
// 			}
// 			columns = append(columns, column{
// 				name:         *c.Name,
// 				typeAffinity: typeAffinity,
// 				notNull:      c.NotNull,
// 			})
// 		}
// 	}

// 	return columns
// }

// func isJoinedTable(stmt *parse.Table) bool {
// 	// check if the table has 2 columns
// 	columns := getColumnsFromStmt(stmt)
// 	if len(columns) != 2 {
// 		return false
// 	}

// 	// check if the table has 2 references
// 	references := getReferencesFromStmt(stmt)
// 	if len(references) != 2 {
// 		return false
// 	}

// 	// check that both columns have each one reference
// 	firstColumnReferenceCount := 0
// 	secondColumnReferenceCount := 0
// 	for _, r := range references {
// 		if r.sourceColumn == columns[0].name {
// 			firstColumnReferenceCount++
// 		}
// 		if r.sourceColumn == columns[1].name {
// 			secondColumnReferenceCount++
// 		}
// 	}
// 	if firstColumnReferenceCount != 1 || secondColumnReferenceCount != 1 {
// 		return false
// 	}

// 	// check that both references target different tables
// 	if references[0].targetTable == references[1].targetTable {
// 		return false
// 	}

// 	return true
// }

type AssociationType int

const (
	Index AssociationType = iota
	Scalar
	OneToOne
	OneToMany
	ManyToOne
	ManyToMany
)

func (a AssociationType) String() string {
	switch a {
	case Index:
		return "Index"
	case Scalar:
		return "Scalar"
	case OneToOne:
		return "OneToOne"
	case OneToMany:
		return "OneToMany"
	case ManyToOne:
		return "ManyToOne"
	case ManyToMany:
		return "ManyToMany"
	}
	return ""
}

type Field struct {
	column          parse.Column
	tableConstraint parse.TableConstraint
	Name            string
	AssociationType AssociationType
	Association     string // either scalar type or object name of association
	NonNull         bool
}

func newField(column parse.Column, tableConstraints []parse.TableConstraint) (*Field, error) {
	field := &Field{
		column:          column,
		Name:            *column.Name,
		AssociationType: Scalar,
		NonNull:         column.NotNull,
	}

	if column.Type != nil {
		field.Association = *column.Type
	}

	if column.PrimaryKey {
		field.AssociationType = Index
	}

	if column.Name != nil && column.ForeignKey != nil && column.ForeignKey.Table != nil && len(column.ForeignKey.Columns) == 1 {
		field.AssociationType = OneToOne
		field.Association = *column.ForeignKey.Table
	}

	for _, constraint := range tableConstraints {
		if constraint.Type == parse.TableConstraintTypeForeignKey && len(constraint.ForeignKeyColumns) == 1 && constraint.ForeignKeyColumns[0] == *column.Name && constraint.ForeignKey != nil && constraint.ForeignKey.Table != nil && len(constraint.ForeignKey.Columns) == 1 {
			field.tableConstraint = constraint
			field.AssociationType = OneToOne
			field.Association = *constraint.ForeignKey.Table
			break
		}
	}

	return field, nil
}

func (f Field) String() string {
	return fmt.Sprintf("Field{Name: %s, AssociationType: %s, Association: %s, NonNull: %t}", f.Name, f.AssociationType, f.Association, f.NonNull)
}

type Object struct {
	stmt   parse.Table
	Name   string
	Fields []Field
}

func newObject(stmt *parse.Table) (*Object, error) {
	obj := &Object{
		stmt: *stmt,
		Name: *stmt.Name,
	}

	for _, column := range stmt.Columns {
		field, err := newField(column, stmt.TableConstraints)
		if err != nil {
			return nil, err
		}
		obj.Fields = append(obj.Fields, *field)
	}

	return obj, nil
}

func (o Object) String() string {
	var s []string
	for _, f := range o.Fields {
		s = append(s, f.String())
	}
	return fmt.Sprintf("Object{Name: %s, Fields: %s}", o.Name, strings.Join(s, ", "))
}

func (o Object) getAssociationTo(name string) *Field {
	for i, field := range o.Fields {
		if (field.AssociationType == OneToOne || field.AssociationType == OneToMany || field.AssociationType == ManyToOne || field.AssociationType == ManyToMany) && field.Association == name {
			return &o.Fields[i]
		}
	}

	return nil
}

func (o Object) getAssociatedObjects() []string {
	if len(o.Fields) == 2 {
		var associatedObjects []string
		for _, field := range o.Fields {
			if field.AssociationType == OneToMany {
				associatedObjects = append(associatedObjects, field.Association)
			}
		}
		if len(associatedObjects) == 2 {
			return associatedObjects
		}
	}

	return nil
}

type Associations struct {
	Objects []Object
}

func Evaluate(sqls []string) (*Associations, error) {
	associations := &Associations{}
	for _, sql := range sqls {
		stmt, err := parse.FromString(sql)
		if err != nil {
			return nil, err
		}
		if stmt.Name == nil {
			return nil, errors.New("Unexpected nil name")
		}

		obj, err := newObject(stmt)
		if err != nil {
			return nil, err
		}
		if obj == nil {
			return nil, errors.New("Unexpected nil object")
		}

		associations.Objects = append(associations.Objects, *obj)
	}

	// change one-to-one to one-to-many where any back-association is missing
	// also add back-association (many-to-one)
	for _, objSource := range associations.Objects {
		for iTarget, objTarget := range associations.Objects {
			forward := objSource.getAssociationTo(objTarget.Name)
			backward := objTarget.getAssociationTo(objSource.Name)
			if forward != nil && backward == nil {
				forward.AssociationType = OneToMany
				associations.Objects[iTarget].Fields = append(associations.Objects[iTarget].Fields, Field{
					AssociationType: ManyToOne,
					Association:     objSource.Name,
				})
			}
		}
	}

	// find joined tables/objects and link the associations
	for _, objJoined := range associations.Objects {
		associatedObjects := objJoined.getAssociatedObjects()
		if len(associatedObjects) == 2 {
			for _, objAssociated := range associations.Objects {
				if objAssociated.Name == objJoined.Name {
					continue
				}

				field := objAssociated.getAssociationTo(objJoined.Name)
				if field != nil {
					field.AssociationType = ManyToMany
					if objAssociated.Name == associatedObjects[0] {
						field.Association = associatedObjects[1]
					} else if objAssociated.Name == associatedObjects[1] {
						field.Association = associatedObjects[0]
					}
				}
			}

			// filter objects to remove the joined object
			// https://github.com/golang/go/wiki/SliceTricks#filter-in-place
			n := 0
			for _, obj := range associations.Objects {
				if obj.Name != objJoined.Name {
					associations.Objects[n] = obj
					n++
				}
			}
			associations.Objects = associations.Objects[:n]
		}
	}

	return associations, nil
}

func (a Associations) String() string {
	var s []string
	for _, obj := range a.Objects {
		s = append(s, obj.String())
	}
	return fmt.Sprintf("Associations{Objects: %s}", strings.Join(s, ", "))
}
