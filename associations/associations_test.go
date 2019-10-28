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
						AssociationType: Index,
						Association:     "INTEGER",
						NonNull:         false,
					},
					Field{
						Name:            "b_id",
						AssociationType: OneToOne,
						Association:     "bs",
						NonNull:         false,
					},
				},
			},
			Object{
				Name: "bs",
				Fields: []Field{
					Field{
						Name:            "id",
						AssociationType: Index,
						Association:     "INTEGER",
						NonNull:         false,
					},
					Field{
						Name:            "a_id",
						AssociationType: OneToOne,
						Association:     "as",
						NonNull:         false,
					},
				},
			},
			Object{
				Name: "cs",
				Fields: []Field{
					Field{
						Name:            "id",
						AssociationType: Index,
						Association:     "INTEGER",
						NonNull:         false,
					},
					Field{
						Name:            "d_id",
						AssociationType: OneToMany,
						Association:     "ds",
						NonNull:         false,
					},
				},
			},
			Object{
				Name: "ds",
				Fields: []Field{
					Field{
						Name:            "id",
						AssociationType: Index,
						Association:     "INTEGER",
						NonNull:         false,
					},
					Field{
						Name:            "",
						AssociationType: ManyToOne,
						Association:     "cs",
						NonNull:         false,
					},
				},
			},
			Object{
				Name: "es",
				Fields: []Field{
					Field{
						Name:            "id",
						AssociationType: Index,
						Association:     "INTEGER",
						NonNull:         false,
					},
					Field{
						Name:            "",
						AssociationType: ManyToOne,
						Association:     "fs",
						NonNull:         false,
					},
				},
			},
			Object{
				Name: "fs",
				Fields: []Field{
					Field{
						Name:            "id",
						AssociationType: Index,
						Association:     "INTEGER",
						NonNull:         false,
					},
					Field{
						Name:            "e_id",
						AssociationType: OneToMany,
						Association:     "es",
						NonNull:         false,
					},
				},
			},
			Object{
				Name: "gs",
				Fields: []Field{
					Field{
						Name:            "id",
						AssociationType: Index,
						Association:     "INTEGER",
						NonNull:         false,
					},
					Field{
						Name:            "",
						AssociationType: ManyToMany,
						Association:     "hs",
						NonNull:         false,
					},
				},
			},
			Object{
				Name: "hs",
				Fields: []Field{
					Field{
						Name:            "id",
						AssociationType: Index,
						Association:     "INTEGER",
						NonNull:         false,
					},
					Field{
						Name:            "",
						AssociationType: ManyToMany,
						Association:     "gs",
						NonNull:         false,
					},
				},
			},
		},
	}

	if diff := deep.Equal(expected, *actual); diff != nil {
		t.Error(diff)
	}
}
