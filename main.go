package main

import (
	"database/sql"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"

	_ "github.com/mattn/go-sqlite3"
)

func main() {
	db, err := sql.Open("sqlite3", "./task.db")
	if err != nil {
		panic(err)
	} else {
		tmpl := template.Must(template.ParseFiles("tasks.html"))

		// http://localhost:8080/tasks?name=e&nameSearchType=contains&status=2&priority=3&idCursorValue=99999&dueDate=2029&otherCursorColumn=due_date&otherCursorValue=2029-03-01&sortOrder=desc
		// http://localhost:8080/tasks?name=e&nameSearchType=contains&status=2&priority=3&idCursorValue=99999&dueDate=2029&otherCursorColumn=due_date&otherCursorValue=2029-03-01&sortColumn=due_date&sortOrder=asc
		http.HandleFunc("/tasks", func(w http.ResponseWriter, r *http.Request) {
			searchPartMap := r.URL.Query()
			tasks := taskSearch(db,
				searchPartMap.Get("name"),
				searchPartMap.Get("nameSearchType"),
				convertQueryParamToUint8(searchPartMap, "priority"),
				convertQueryParamToUint8(searchPartMap, "status"),
				searchPartMap.Get("dueDate"),
				convertQueryParamToUint32(searchPartMap, "idCursorValue"),
				searchPartMap.Get("otherCursorColumn"),
				searchPartMap.Get("otherCursorValue"),
				searchPartMap.Get("sortOrder"),
			)

			queryParams := make(map[string]string)
			for k, vals := range r.URL.Query() {
				queryParams[k] = vals[0] // take first value when multiple exist
				fmt.Println(k, vals[0])
			}

			fmt.Println(tasks)

			data := TasksPageData{Tasks: tasks, QueryParams: queryParams, Priorities: priorities, Statuses: statuses, SortColumnSelectOptions: sortColumnSelectOptions}

			tmpl.Execute(w, data)
		})

		http.HandleFunc("/task-count", func(w http.ResponseWriter, r *http.Request) {
			var searchPartMap url.Values = r.URL.Query()
			countTasks(db,
				searchPartMap.Get("name"),
				searchPartMap.Get("nameSearchType"),
				convertQueryParamToUint8(searchPartMap, "priority"),
				convertQueryParamToUint8(searchPartMap, "status"),
				searchPartMap.Get("dueDate"),
			)
		})

		http.ListenAndServe(":8080", nil)

		defer db.Close()
	}
}

func convertQueryParamToUint8(searchPartMap url.Values, paramName string) uint8 {
	num, err := strconv.ParseUint(searchPartMap.Get(paramName), 10, 8)
	if err != nil {
		return 0
	}
	return uint8(num)
}

func convertQueryParamToUint32(searchPartMap url.Values, paramName string) uint32 {
	num, err := strconv.ParseUint(searchPartMap.Get(paramName), 10, 32)
	if err != nil {
		return 0
	}
	return uint32(num)
}

// TODO: create constants/"enums" for nameSearchType and dueSearchType
func taskSearch(db *sql.DB, name string, nameSearchType string, priority uint8, status uint8, dueDate string,
	idCursorValue uint32, otherCursorColumn string, otherCursorValue string, sortOrder string) []Task {
	switch nameSearchType {
	case "startsWith":
		name += "%"
	case "endsWith":
		name = "%" + name
	default:
		name = "%" + name + "%"
	}

	dueDate += "%"

	var priorityClause string
	switch priority {
	case 1:
		priorityClause = "= 1"
	case 2:
		priorityClause = "= 2"
	case 3:
		priorityClause = "= 3"
	default:
		priorityClause = "IN (1, 2, 3)"
	}

	var statusClause string
	switch status {
	case 1:
		statusClause = "= 1"
	case 2:
		statusClause = "= 2"
	case 3:
		statusClause = "= 3"
	case 4:
		statusClause = "= 4"
	default:
		statusClause = "IN (1, 2, 3, 4)"
	}

	cursorColumns := []string{"name", "priority", "status", "due_date"}
	if !slices.Contains(cursorColumns, strings.ToLower(otherCursorColumn)) {
		otherCursorColumn = "name"
	}

	var cursorOperator string
	if strings.ToUpper(sortOrder) == "DESC" {
		cursorOperator = "<"
		sortOrder = "DESC"
	} else {
		cursorOperator = ">"
		sortOrder = "ASC"
	}

	sql := `SELECT id, name, priority, status, due_date FROM tasks
	WHERE name LIKE ?
	AND priority %s
	AND status %s
	AND due_date LIKE ?
	AND (%s, id) %s (?, ?)
	ORDER BY %s %s, id
	LIMIT 25`

	fmt.Println(fmt.Sprintf(sql, priorityClause, statusClause, otherCursorColumn, cursorOperator, otherCursorColumn, sortOrder))
	fmt.Println(name, dueDate, otherCursorValue, idCursorValue)

	rows, err := db.Query(fmt.Sprintf(sql, priorityClause, statusClause, otherCursorColumn, cursorOperator, otherCursorColumn, sortOrder),
		name, dueDate, otherCursorValue, idCursorValue)
	if err != nil {
		panic(err)
	}

	var tasks []Task
	for rows.Next() {
		var task Task
		err = rows.Scan(&task.ID, &task.Name, &task.Priority, &task.Status, &task.DueDate)
		if err != nil {
			panic(err)
		}
		tasks = append(tasks, task)
	}
	return tasks
}

func countTasks(db *sql.DB, name string, nameSearchType string, priority uint8, status uint8, dueDate string) uint32 {
	switch nameSearchType {
	case "startsWith":
		name += "%"
	case "endsWith":
		name = "%" + name
	default:
		name = "%" + name + "%"
	}

	var priorityClause string
	priorities := []uint8{1, 2, 3}
	if slices.Contains(priorities, priority) {
		priorityClause = "= " + strconv.Itoa(int(priority))
	} else {
		priorityClause = "IN (1, 2, 3)"
	}

	var statusClause string
	statuses := []uint8{1, 2, 3, 4}
	if slices.Contains(statuses, status) {
		statusClause = "= " + strconv.Itoa(int(status))
	} else {
		statusClause = "IN (1, 2, 3, 4)"
	}

	dueDate += "%"

	sql := `SELECT COUNT(*) FROM tasks
	WHERE name LIKE ?
	AND priority %s
	AND status %s
	AND due_date LIKE ?`

	var count uint32
	err := db.QueryRow(fmt.Sprintf(sql, priorityClause, statusClause), name, dueDate).Scan(&count)
	if err != nil {
		panic(err)
	}
	fmt.Printf("Task Count: %d\n", count)
	return count
}

type Task struct {
	ID       uint32
	Name     string
	Priority uint8
	Status   uint8
	DueDate  string // TODO: change to a "Date" type
}

func (t Task) string() string {
	return fmt.Sprintf("%d %v %d %d %v", t.ID, t.Name, t.Priority, t.Status, t.DueDate)
}

type TasksPageData struct {
	Tasks                   []Task
	QueryParams             map[string]string
	Priorities              []selectOption
	Statuses                []selectOption
	SortColumnSelectOptions []selectOption
}

const (
	ALL = iota
	LOW
	MEDIUM
	HIGH
)

const (
	NEW     = 1
	STARTED = 2
	BLOCKED = 3
	DONE    = 4
)

type selectOption struct {
	Value      string
	TagContent string
}

// ID              string
// QueryParamValue string

var all selectOption = selectOption{Value: "0", TagContent: "ALL"}

// priorites
var low selectOption = selectOption{"1", "LOW"}
var medium selectOption = selectOption{"2", "MEDIUM"}
var high selectOption = selectOption{"3", "HIGH"}
var priorities = []selectOption{all, low, medium, high}

// statuses
var newStatus selectOption = selectOption{"1", "NEW"}
var started selectOption = selectOption{"2", "STARTED"}
var blocked selectOption = selectOption{"3", "BLOCKED"}
var done selectOption = selectOption{"4", "DONE"}
var statuses = []selectOption{all, newStatus, started, blocked, done}

// sort columns
var dueDateColumnSelect selectOption = selectOption{"due_date", "Due Date"}
var nameColumnSelectOption selectOption = selectOption{"name", "Name"}
var priorityColumnSelectOption selectOption = selectOption{"priority", "Priority"}
var statusColumnSelectOption selectOption = selectOption{"status", "Status"}
var sortColumnSelectOptions = []selectOption{dueDateColumnSelect, nameColumnSelectOption, priorityColumnSelectOption, statusColumnSelectOption}
