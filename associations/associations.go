package associations

import (
	"fmt"
	"strings"

	parse "github.com/h3ndrk/go-sqlite-createtable-parser"
	"github.com/pkg/errors"
)

// AssociationType represents the type of a association
type AssociationType int

const (
	// Identification means that the field holds an identifier
	Identification AssociationType = iota
	// Scalar means that the field holds a normal scalar
	Scalar
	// OneToOne means that the field associates a one-to-one association
	OneToOne
	// OneToMany means that the field associates a one-to-many association
	OneToMany
	// ManyToOne means that the field associates a many-to-one association
	ManyToOne
	// ManyToMany means that the field associates a many-to-many association
	ManyToMany
)

func (a AssociationType) String() string {
	switch a {
	case Identification:
		return "Identification"
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

// Field represents an object field which holds either a identification, scalar or object association
type Field struct {
	column          parse.Column
	tableConstraint parse.TableConstraint
	Name            string
	AssociationType AssociationType
	Association     string // either scalar type or object name of association
	NonNull         bool
	ForeignField    *string
	JoinTable       *string
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
		field.AssociationType = Identification
	}

	if column.Name != nil && column.ForeignKey != nil && column.ForeignKey.Table != nil && len(column.ForeignKey.Columns) == 1 {
		field.AssociationType = OneToOne
		field.Association = *column.ForeignKey.Table
	}

	for _, constraint := range tableConstraints {
		// check for primary key
		if constraint.Type == parse.TableConstraintTypePrimaryKey &&
			len(constraint.IndexedColumns) == 1 &&
			constraint.IndexedColumns[0].Name != nil &&
			column.Name != nil &&
			*constraint.IndexedColumns[0].Name == *column.Name {

			field.tableConstraint = constraint
			field.AssociationType = Identification
			break
		}

		// check for foreign key
		if constraint.Type == parse.TableConstraintTypeForeignKey &&
			len(constraint.ForeignKeyColumns) == 1 &&
			column.Name != nil &&
			constraint.ForeignKeyColumns[0] == *column.Name &&
			constraint.ForeignKey != nil &&
			constraint.ForeignKey.Table != nil &&
			len(constraint.ForeignKey.Columns) == 1 {

			field.tableConstraint = constraint
			field.AssociationType = OneToOne
			field.Association = *constraint.ForeignKey.Table
			break
		}
	}

	return field, nil
}

func (f Field) String() string {
	foreignField := "null"
	if f.ForeignField != nil {
		foreignField = *f.ForeignField
	}
	joinTable := "null"
	if f.JoinTable != nil {
		joinTable = *f.ForeignField
	}
	return fmt.Sprintf("Field{Name: %s, AssociationType: %s, Association: %s, NonNull: %t, ForeignField: %s, JoinTable: %s}", f.Name, f.AssociationType, f.Association, f.NonNull, foreignField, joinTable)
}

// Object represents an object which has fields with associations to other objects
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

// Associations holds the list of objects
type Associations struct {
	Objects []Object
}

// Evaluate parses all SQL strings and generates an associated list of objects
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
					ForeignField:    &forward.Name,
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
					field.JoinTable = &objJoined.Name
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
