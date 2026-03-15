package imgopt_db

import (
	"fmt"
	"reflect"
	"strings"
)

// [*sql.Rows], [*sql.Row]
type DatabaseRow interface {
	Scan(dest ...any) error
}

type DatabaseEntity interface {
	ScanFullRow(row DatabaseRow) error
}

// сохраняет entity в базе данных:
// если entity.Id == 0, то использует INSERT, иначе использует UPDATE по entity.Id (обновляет все поля)
// entity обязательно должна быть struct'ом и иметь первым полем  Id int `json:"id"`
// все остальные поля должны отражать структуру таблицы и быть экспортируемыми (начинаются с заглавной буквы)
// с помощью тега `json:"fieldname"` вместо fieldname указывается название столбца в таблице
// возвращает id сохранённой записи и ошибку, если есть
func (dw *DatabaseWrapper) SaveEntity(table string, entity any) (int, error) {
	db := dw.DB

	fields, values := getEntityFields(entity)
	fieldsWithoutId, valuesWithoutId := fields[1:], values[1:]

	exmarks := make([]string, len(fieldsWithoutId))
	for i := range len(fieldsWithoutId) {
		exmarks[i] = "?"
	}

	entityId := values[0]
	if t := reflect.TypeOf(values[0]); t.Kind() != reflect.Int {
		err := fmt.Errorf("SaveEntity (%v): Invalid entity ID: expected int, got %v", table, t.Kind())
		return 0, err
	}

	if entityId == 0 {
		sql := fmt.Sprintf("INSERT INTO %v (%v) VALUES(%v)", table, strings.Join(fieldsWithoutId, ", "), strings.Join(exmarks, ", "))

		stmt, err := db.Prepare(sql)
		if err != nil {
			return 0, err
		}

		r, err := stmt.Exec(valuesWithoutId...)
		if err != nil {
			return 0, err
		}

		insertedId, _ := r.LastInsertId()

		return int(insertedId), err
	}

	for i := range len(exmarks) {
		exmarks[i] = fmt.Sprintf("%v = ?", fieldsWithoutId[i])
	}
	sql := fmt.Sprintf("UPDATE %v SET %v WHERE id = ?", table, strings.Join(exmarks, ", "))

	stmt, err := db.Prepare(sql)
	if err != nil {
		return 0, err
	}

	args := make([]any, len(values)+1)
	args = append(args, valuesWithoutId...)
	args = append(args, entityId)

	r, err := stmt.Exec(args...)
	insertedId, err := r.LastInsertId()

	return int(insertedId), err
}

// выносит все названия полей в []string, а значения этих полей в []any
// по индексам, название поля из []string и значение из []any совпадают
func getEntityFields(entity any) ([]string, []any) {
	t := reflect.TypeOf(entity)
	v := reflect.ValueOf(entity)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
		v = v.Elem()
	}
	if t.Kind() != reflect.Struct {
		fmt.Printf("getEntityFields: expected struct, got %v (%v)", t, entity)
		return nil, nil
	}

	var keys []string
	var values []any

	for i := range t.NumField() {
		field := t.Field(i)
		if field.PkgPath != "" {
			continue
		}

		if field.Type.Kind() == reflect.Struct {
			nestedKeys, nestedValues := getEntityFields(v.Field(i).Interface())
			keys = append(keys, nestedKeys...)
			values = append(values, nestedValues...)
		} else {
			key := field.Name
			if tag := field.Tag.Get("json"); tag != "" {
				name := strings.Split(tag, ",")[0]
				if name == "-" {
					continue
				}
				if name != "" {
					key = name
				}
			}

			keys = append(keys, key)
			values = append(values, v.Field(i).Interface())
		}
	}

	return keys, values
}
