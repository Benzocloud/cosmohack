package repository

import _ "embed"

//go:embed sql/area_insert.sql
var queryInsertArea string

//go:embed sql/area_get.sql
var queryGetArea string

//go:embed sql/area_list.sql
var queryListAreas string
