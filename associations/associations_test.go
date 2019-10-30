package associations

import (
	"testing"

	"github.com/go-test/deep"
	"github.com/stretchr/testify/assert"
)

func TestAssociations(t *testing.T) {
	actual, err := Evaluate([]string{
		// one to one
		"CREATE TABLE as (id INTEGER PRIMARY KEY, b_id INTEGER REFERENCES bs(id));",
		"CREATE TABLE bs (id INTEGER PRIMARY KEY, a_id INTEGER REFERENCES as(id));",

		// one to many
		"CREATE TABLE cs (id INTEGER PRIMARY KEY, d_id INTEGER REFERENCES ds(id));",
		"CREATE TABLE ds (id INTEGER PRIMARY KEY);",

		// many to one
		"CREATE TABLE es (id INTEGER PRIMARY KEY);",
		"CREATE TABLE fs (id INTEGER PRIMARY KEY, e_id INTEGER REFERENCES es(id));",

		// many to many
		"CREATE TABLE gs (id INTEGER PRIMARY KEY);",
		"CREATE TABLE hs (id INTEGER PRIMARY KEY);",
		"CREATE TABLE g_h (g_id INTEGER REFERENCES gs(id), h_id INTEGER REFERENCES hs(id));",
	})
	assert.NoError(t, err)
	assert.NotNil(t, actual)

	expected := Associations{
		Objects: []Object{
			Object{
				Name: "as",
				Fields: []Field{
					Field{
						Name:            "id",
						AssociationType: Identification,
						Association:     "INTEGER",
						NonNull:         false,
						ForeignField:    nil,
						JoinTable:       nil,
					},
					Field{
						Name:            "b_id",
						AssociationType: OneToOne,
						Association:     "bs",
						NonNull:         false,
						ForeignField:    nil,
						JoinTable:       nil,
					},
				},
			},
			Object{
				Name: "bs",
				Fields: []Field{
					Field{
						Name:            "id",
						AssociationType: Identification,
						Association:     "INTEGER",
						NonNull:         false,
						ForeignField:    nil,
						JoinTable:       nil,
					},
					Field{
						Name:            "a_id",
						AssociationType: OneToOne,
						Association:     "as",
						NonNull:         false,
						ForeignField:    nil,
						JoinTable:       nil,
					},
				},
			},
			Object{
				Name: "cs",
				Fields: []Field{
					Field{
						Name:            "id",
						AssociationType: Identification,
						Association:     "INTEGER",
						NonNull:         false,
						ForeignField:    nil,
						JoinTable:       nil,
					},
					Field{
						Name:            "d_id",
						AssociationType: OneToMany,
						Association:     "ds",
						NonNull:         false,
						ForeignField:    nil,
						JoinTable:       nil,
					},
				},
			},
			Object{
				Name: "ds",
				Fields: []Field{
					Field{
						Name:            "id",
						AssociationType: Identification,
						Association:     "INTEGER",
						NonNull:         false,
						ForeignField:    nil,
						JoinTable:       nil,
					},
					Field{
						Name:            "",
						AssociationType: ManyToOne,
						Association:     "cs",
						NonNull:         false,
						ForeignField:    func() *string { s := "d_id"; return &s }(),
						JoinTable:       nil,
					},
				},
			},
			Object{
				Name: "es",
				Fields: []Field{
					Field{
						Name:            "id",
						AssociationType: Identification,
						Association:     "INTEGER",
						NonNull:         false,
						ForeignField:    nil,
						JoinTable:       nil,
					},
					Field{
						Name:            "",
						AssociationType: ManyToOne,
						Association:     "fs",
						NonNull:         false,
						ForeignField:    func() *string { s := "e_id"; return &s }(),
						JoinTable:       nil,
					},
				},
			},
			Object{
				Name: "fs",
				Fields: []Field{
					Field{
						Name:            "id",
						AssociationType: Identification,
						Association:     "INTEGER",
						NonNull:         false,
						ForeignField:    nil,
						JoinTable:       nil,
					},
					Field{
						Name:            "e_id",
						AssociationType: OneToMany,
						Association:     "es",
						NonNull:         false,
						ForeignField:    nil,
						JoinTable:       nil,
					},
				},
			},
			Object{
				Name: "gs",
				Fields: []Field{
					Field{
						Name:            "id",
						AssociationType: Identification,
						Association:     "INTEGER",
						NonNull:         false,
						ForeignField:    nil,
						JoinTable:       nil,
					},
					Field{
						Name:            "",
						AssociationType: ManyToMany,
						Association:     "hs",
						NonNull:         false,
						ForeignField:    func() *string { s := "g_id"; return &s }(),
						JoinTable:       func() *string { s := "g_h"; return &s }(),
					},
				},
			},
			Object{
				Name: "hs",
				Fields: []Field{
					Field{
						Name:            "id",
						AssociationType: Identification,
						Association:     "INTEGER",
						NonNull:         false,
						ForeignField:    nil,
						JoinTable:       nil,
					},
					Field{
						Name:            "",
						AssociationType: ManyToMany,
						Association:     "gs",
						NonNull:         false,
						ForeignField:    func() *string { s := "h_id"; return &s }(),
						JoinTable:       func() *string { s := "g_h"; return &s }(),
					},
				},
			},
		},
	}

	if diff := deep.Equal(expected, *actual); diff != nil {
		t.Error(diff)
	}
}
