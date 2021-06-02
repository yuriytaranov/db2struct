package db2struct

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strings"
)

// GetColumnsFromMysqlTable Select column details from information schema and return map of map
func GetColumnsFromMysqlTable(mariadbUser string, mariadbPassword string, mariadbHost string, mariadbPort int, mariadbDatabase string, mariadbTable string) (*map[string]map[string]string, []string, error) {
	dsn := fmt.Sprintf("@tcp(%s:%d)/%s?&parseTime=True",
		mariadbHost,
		mariadbPort,
		mariadbDatabase)
	if mariadbPassword != "" {
		dsn = fmt.Sprintf("%s:%s%s", mariadbUser, mariadbPassword, dsn)
	} else {
		dsn = fmt.Sprintf("%s%s", mariadbUser, dsn)
	}
	db, err := sql.Open("mysql", dsn)

	// Check for error in db, note this does not check connectivity but does check uri
	if err != nil {
		fmt.Println("Error opening mysql db: " + err.Error())
		return nil, nil, err
	}
	defer func() {
		err := db.Close()
		log.Fatalln(err)
	}()
	columnNamesSorted := []string{}

	// Store colum as map of maps
	columnDataTypes := make(map[string]map[string]string)
	// Select columnd data from INFORMATION_SCHEMA
	columnDataTypeQuery := "SELECT COLUMN_NAME, COLUMN_KEY, DATA_TYPE, IS_NULLABLE, COLUMN_COMMENT FROM INFORMATION_SCHEMA.COLUMNS WHERE TABLE_SCHEMA = ? AND table_name = ? order by ordinal_position asc"

	if Debug {
		fmt.Println("running: " + columnDataTypeQuery)
	}

	rows, err := db.Query(columnDataTypeQuery, mariadbDatabase, mariadbTable)

	if err != nil {
		fmt.Println("Error selecting from db: " + err.Error())
		return nil, nil, err
	}
	if rows != nil {
		defer func() {
			err := rows.Close()
			if err != nil {
				log.Println(err)
			}
		}()
	} else {
		return nil, nil, errors.New("no results returned for table")
	}

	for rows.Next() {
		var column string
		var columnKey string
		var dataType string
		var nullable string
		var comment string
		rows.Scan(&column, &columnKey, &dataType, &nullable, &comment)

		columnDataTypes[column] = map[string]string{"value": dataType, "nullable": nullable, "primary": columnKey, "comment": comment}
		columnNamesSorted = append(columnNamesSorted, column)
	}

	return &columnDataTypes, columnNamesSorted, err
}

// Generate go struct entries for a map[string]interface{} structure
func generateMysqlTypes(
	obj map[string]map[string]string,
	columnsSorted []string,
	depth int,
	jsonAnnotation bool,
	gormAnnotation bool,
	gureguTypes bool,
	dbAnnotations bool,
	structName string) string {
	structure := "struct {"

	for _, key := range columnsSorted {
		mysqlType := obj[key]
		nullable := false
		if mysqlType["nullable"] == "YES" {
			nullable = true
		}

		primary := ""
		if mysqlType["primary"] == "PRI" {
			primary = ";primary_key"
		}

		// Get the corresponding go value type for this mysql type
		// If the guregu (https://github.com/guregu/null) CLI option is passed use its types, otherwise use go's sql.NullX

		valueType := mysqlTypeToGoType(mysqlType["value"], nullable, gureguTypes)

		fieldName := fmtFieldName(strings.ToLower(stringifyFirstChar(key)))
		var annotations []string
		if gormAnnotation {
			annotations = append(annotations, fmt.Sprintf("gorm:\"column:%s%s\"", key, primary))
		}
		if dbAnnotations {
			annotations = append(annotations, fmt.Sprintf("db:\"%s%s\"", structName, fieldName))
		}
		if jsonAnnotation {
			annotations = append(annotations, fmt.Sprintf("json:\"%s\"", key))
		}

		// add column comment
		comment := mysqlType["comment"]
		if len(annotations) > 0 {
			annotate := fmt.Sprintf("\n%s %s `%s`", fieldName, valueType, strings.Join(annotations, " "))
			if len(comment) > 0 {
				annotate = fmt.Sprintf("%s // %s", annotate, comment)
			}
			structure += annotate
		} else {
			structure += fmt.Sprintf("\n%s %s", fieldName, valueType)
		}
	}
	return structure
}

// mysqlTypeToGoType converts the mysql types to go compatible sql.Nullable (https://golang.org/pkg/database/sql/) types
func mysqlTypeToGoType(mysqlType string, nullable bool, gureguTypes bool) string {
	switch mysqlType {
	case "tinyint", "int", "smallint", "mediumint":
		if nullable {
			if gureguTypes {
				return gureguNullInt
			}
			return sqlNullInt
		}
		return golangInt
	case "bigint":
		if nullable {
			if gureguTypes {
				return gureguNullInt
			}
			return sqlNullInt
		}
		return golangInt64
	case "char", "enum", "varchar", "longtext", "mediumtext", "text", "tinytext", "json":
		if nullable {
			if gureguTypes {
				return gureguNullString
			}
			return sqlNullString
		}
		return "string"
	case "date", "datetime", "time", "timestamp":
		if nullable && gureguTypes {
			return gureguNullTime
		}
		return golangTime
	case "decimal", "double":
		if nullable {
			if gureguTypes {
				return gureguNullFloat
			}
			return sqlNullFloat
		}
		return golangFloat64
	case "float":
		if nullable {
			if gureguTypes {
				return gureguNullFloat
			}
			return sqlNullFloat
		}
		return golangFloat32
	case "binary", "blob", "longblob", "mediumblob", "varbinary":
		return golangByteArray
	}
	return ""
}
