// Copyright 2015-2017, Cyrill @ Schumacher.fm and the CoreStore contributors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package dml

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"testing"

	"github.com/corestoreio/errors"
	_ "github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

///////////////////////////////////////////////////////////////////////////////
// TEST HELPERS
///////////////////////////////////////////////////////////////////////////////

// Returns a session that's not backed by a database
func createFakeSession() *ConnPool {
	cxn, err := NewConnPool()
	if err != nil {
		panic(err)
	}
	return cxn
}

func createRealSession(t testing.TB) *ConnPool {
	dsn := os.Getenv("CS_DSN")
	if dsn == "" {
		t.Skip("Environment variable CS_DSN not found. Skipping ...")
	}
	cxn, err := NewConnPool(
		WithDSN(dsn),
	)
	if err != nil {
		t.Fatal(err)
	}
	return cxn
}

func createRealSessionWithFixtures(t testing.TB, c *installFixturesConfig) *ConnPool {
	sess := createRealSession(t)
	installFixtures(sess.DB, c)
	return sess
}

var _ ColumnMapper = (*dmlPerson)(nil)
var _ LastInsertIDAssigner = (*dmlPerson)(nil)
var _ ColumnMapper = (*dmlPersons)(nil)

var _ ColumnMapper = (*nullTypedRecord)(nil)

type dmlPerson struct {
	ID    uint64
	Name  string
	Email NullString
	Key   NullString
}

func (p *dmlPerson) AssignLastInsertID(id int64) {
	p.ID = uint64(id)
}

// RowScan loads a single row from a SELECT statement returning only one row
func (p *dmlPerson) MapColumns(cm *ColumnMap) error {
	if cm.Mode() == 'a' {
		// TODO(CyS) jump into this case when `select *` or `select
		// col1,col2...` returns all columns!!! not possible due to missing
		// Index() call.
		return cm.Uint64(&p.ID).String(&p.Name).NullString(&p.Email).NullString(&p.Key).Err()
	}
	for cm.Next() {
		// TODO: numbers are experimental and in case all columns are requested,
		// as we don't know the column names numbering them would be enough.
		// this should avoid the above AllColumns `if` branch.
		c := cm.Column()
		switch c {
		case "id", "0":
			cm.Uint64(&p.ID)
		case "name", "1":
			cm.String(&p.Name)
		case "email", "2":
			cm.NullString(&p.Email)
		case "key", "3":
			cm.NullString(&p.Key)
		case "store_id", "created_at", "total_income":
			// noop don't trigger the default case
		default:
			return errors.NewNotFoundf("[dml_test] dmlPerson Column %q not found", c)
		}
	}
	return errors.WithStack(cm.Err())
}

type dmlPersons struct {
	Data []*dmlPerson
}

func (ps *dmlPersons) IDs(ret ...uint64) []uint64 {
	if ret == nil {
		ret = make([]uint64, 0, len(ps.Data))
	}
	for _, p := range ps.Data {
		ret = append(ret, p.ID)
	}
	return ret
}

func (ps *dmlPersons) Names(ret ...string) []string {
	if ret == nil {
		ret = make([]string, 0, len(ps.Data))
	}
	for _, p := range ps.Data {
		ret = append(ret, p.Name)
	}
	return ret
}

func (ps *dmlPersons) Emails(ret ...NullString) []NullString {
	if ret == nil {
		ret = make([]NullString, 0, len(ps.Data))
	}
	for _, p := range ps.Data {
		ret = append(ret, p.Email)
	}
	return ret
}

// MapColumns gets called in the `for rows.Next()` loop each time in case of IsNew
func (ps *dmlPersons) MapColumns(cm *ColumnMap) error {
	switch m := cm.Mode(); m {
	case 'a', 'R': // INSERT STATEMENT requesting all columns aka arguments
		for _, p := range ps.Data {
			if err := p.MapColumns(cm); err != nil {
				return errors.WithStack(err)
			}
		}
	case 'w':
		// case for scanning when loading certain rows, hence we write data from
		// the DB into the struct in each for-loop.
		if cm.Count == 0 {
			ps.Data = ps.Data[:0]
		}
		p := new(dmlPerson)
		if err := p.MapColumns(cm); err != nil {
			return errors.WithStack(err)
		}
		ps.Data = append(ps.Data, p)
	case 'r':
		// SELECT, DELETE or UPDATE or INSERT with n columns
		for cm.Next() {
			switch c := cm.Column(); c {
			case "id":
				cm.Args = cm.Args.Uint64s(ps.IDs()...)
			case "name":
				cm.Args = cm.Args.Strings(ps.Names()...)
			case "email":
				cm.Args = cm.Args.NullString(ps.Emails()...)
			default:
				return errors.NewNotFoundf("[dml_test] dmlPerson Column %q not found", c)
			}
		}
	default:
		return errors.NewNotSupportedf("[dml] Unknown Mode: %q", string(m))
	}
	return cm.Err()
}

//func (ps *dmlPersons) AssignLastInsertID(uint64) error {
//	// todo iterate and assign to the last item in the slice and assign
//	// decremented IDs to the previous items in the slice.
//	return nil
//}
//

type nullTypedRecord struct {
	ID         int64
	StringVal  NullString
	Int64Val   NullInt64
	Float64Val NullFloat64
	TimeVal    NullTime
	BoolVal    NullBool
}

func (p *nullTypedRecord) MapColumns(cm *ColumnMap) error {
	if cm.Mode() == 'a' {
		return cm.Int64(&p.ID).NullString(&p.StringVal).NullInt64(&p.Int64Val).NullFloat64(&p.Float64Val).NullTime(&p.TimeVal).NullBool(&p.BoolVal).Err()
	}
	for cm.Next() {
		c := cm.Column()
		switch c {
		case "id":
			cm.Int64(&p.ID)
		case "string_val":
			cm.NullString(&p.StringVal)
		case "int64_val":
			cm.NullInt64(&p.Int64Val)
		case "float64_val":
			cm.NullFloat64(&p.Float64Val)
		case "time_val":
			cm.NullTime(&p.TimeVal)
		case "bool_val":
			cm.NullBool(&p.BoolVal)
		default:
			return errors.NewNotFoundf("[dml_test] Column %q not found", c)
		}
	}
	return cm.Err()
}

type installFixturesConfig struct {
	AddPeopleWithMaxUint64 bool
}

func installFixtures(db *sql.DB, c *installFixturesConfig) {
	createPeopleTable := fmt.Sprintf(`
		CREATE TABLE dml_people (
			id bigint(8) unsigned NOT NULL auto_increment PRIMARY KEY,
			name varchar(255) NOT NULL,
			email varchar(255),
			%s varchar(255),
			store_id smallint(5) unsigned DEFAULT 0 COMMENT 'Store Id',
			created_at timestamp NOT NULL DEFAULT '0000-00-00 00:00:00' COMMENT 'Created At',
			total_income decimal(12,4) NOT NULL DEFAULT 0.0000 COMMENT 'Total Income Amount'
		)
	`, "`key`")

	createNullTypesTable := `
		CREATE TABLE null_types (
			id int(11) NOT NULL auto_increment PRIMARY KEY,
			string_val varchar(255) NULL,
			int64_val int(11) NULL,
			float64_val float NULL,
			time_val datetime NULL,
			bool_val bool NULL
		)
	`
	// see also test case "LoadUint64 max Uint64 found"
	sqlToRun := []string{
		"DROP TABLE IF EXISTS dml_people",
		createPeopleTable,
		"INSERT INTO dml_people (name,email) VALUES ('Jonathan', 'jonathan@uservoice.com')",
		"INSERT INTO dml_people (name,email) VALUES ('Dmitri', 'zavorotni@jadius.com')",

		"DROP TABLE IF EXISTS null_types",
		createNullTypesTable,
	}
	if c != nil && c.AddPeopleWithMaxUint64 {
		sqlToRun = append(sqlToRun, "INSERT INTO dml_people (id,name,email) VALUES (18446744073700551613,'Cyrill', 'firstname@lastname.fm')")
	}

	for _, v := range sqlToRun {
		_, err := db.Exec(v)
		if err != nil {
			log.Fatalln("Failed to execute statement: ", v, " Got error: ", err)
		}
	}
}

var _ Querier = (*dbMock)(nil)
var _ Execer = (*dbMock)(nil)

type dbMock struct {
	error
	prepareFn func(query string) (*sql.Stmt, error)
}

func (pm dbMock) PrepareContext(ctx context.Context, query string) (*sql.Stmt, error) {
	if pm.error != nil {
		return nil, pm.error
	}
	return pm.prepareFn(query)
}

func (pm dbMock) QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	if pm.error != nil {
		return nil, pm.error
	}
	return nil, nil
}

func (pm dbMock) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	if pm.error != nil {
		return nil, pm.error
	}
	return nil, nil
}

// compareToSQL compares a SQL object with a placeholder string and an optional
// interpolated string. This function also exists in file dml_public_test.go to
// avoid import cycles when using a single package dedicated for testing.
func compareToSQL(
	t testing.TB, qb QueryBuilder, wantErr errors.BehaviourFunc,
	wantSQLPlaceholders, wantSQLInterpolated string,
	wantArgs ...interface{},
) {

	sqlStr, args, err := qb.ToSQL()
	if wantErr == nil {
		require.NoError(t, err)
	} else {
		require.True(t, wantErr(err), "%+v", err)
	}

	if wantSQLPlaceholders != "" {
		assert.Equal(t, wantSQLPlaceholders, sqlStr, "Placeholder SQL strings do not match")
		assert.Equal(t, wantArgs, args, "Placeholder Arguments do not match")
	}

	if wantSQLInterpolated == "" {
		return
	}

	// If you care regarding the duplication ... send us a PR ;-)
	// Enables Interpolate feature and resets it after the test has been
	// executed.
	switch dml := qb.(type) {
	case *Delete:
		dml.Interpolate()
		defer func() { dml.IsInterpolate = false }()
	case *Update:
		dml.Interpolate()
		defer func() { dml.IsInterpolate = false }()
	case *Insert:
		dml.Interpolate()
		defer func() { dml.IsInterpolate = false }()
	case *Select:
		dml.Interpolate()
		defer func() { dml.IsInterpolate = false }()
	case *Union:
		dml.Interpolate()
		defer func() { dml.IsInterpolate = false }()
	case *With:
		dml.Interpolate()
		defer func() { dml.IsInterpolate = false }()
	case *Show:
		dml.Interpolate()
		defer func() { dml.IsInterpolate = false }()
	default:
		t.Fatalf("func compareToSQL: the type %#v is not (yet) supported.", qb)
	}

	sqlStr, args, err = qb.ToSQL() // Call with enabled interpolation
	require.Nil(t, args, "Arguments should be nil when the SQL string gets interpolated")
	if wantErr == nil {
		require.NoError(t, err)
	} else {
		require.True(t, wantErr(err), "%+v")
	}
	require.Equal(t, wantSQLInterpolated, sqlStr, "Interpolated SQL strings do not match")
}
