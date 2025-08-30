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
		http.HandleFunc("/tasks", func(w http.ResponseWriter, r *http.Request) {
			searchPartMap := r.URL.Query()
			name := searchPartMap.Get("name")
			nameSearchType := searchPartMap.Get("nameSearchType")
			priority := convertQueryParamToUint8(searchPartMap, "priority")
			status := convertQueryParamToUint8(searchPartMap, "status")
			dueDate := searchPartMap.Get("dueDate")
			idCursorValue := convertQueryParamToUint32(searchPartMap, "idCursorValue")
			otherCursorColumn := searchPartMap.Get("otherCursorColumn")
			otherCursorValue := searchPartMap.Get("otherCursorValue")
			sortOrder := searchPartMap.Get("sortOrder")

			var tasks []Task
			if idCursorValue == 0 || otherCursorValue == "" {
				tasks = fetchFirst25Tasks(db, name, nameSearchType, priority, status, dueDate, otherCursorColumn, sortOrder)
				numTasksFound := len(tasks)

				if numTasksFound > 0 {
					lastTask := tasks[len(tasks)-1]
					switch otherCursorColumn {
					case "due_date":
						otherCursorValue = lastTask.DueDate
					case "status":
						otherCursorValue = strconv.Itoa(int(lastTask.Status))
					case "priority":
						otherCursorValue = strconv.Itoa(int(lastTask.Priority))
					default:
						otherCursorValue = lastTask.Name
					}
				}

				w.Header().Set("Hx-Trigger", "countTasks")
			} else {
				tasks = fetchNext25Tasks(db, name, nameSearchType, priority, status, dueDate, idCursorValue, otherCursorColumn, otherCursorValue, sortOrder)
			}

			queryParams := make(map[string]string)
			for k, vals := range r.URL.Query() {
				queryParams[k] = vals[0] // take first value when multiple exist
			}

			if r.Header.Get("Hx-Request") != "true" {
				tmpl, err := template.ParseFiles("tasks.html")
				if err != nil {
					panic(err)
				}
				data := TasksPageData{Tasks: tasks, QueryParams: queryParams, Priorities: priorities, Statuses: statuses, SortColumnSelectOptions: sortColumnSelectOptions, OtherCursorValueStr: otherCursorValue}
				tmpl.Execute(w, data)
			} else {
				row := `
					<tr>
						<td>%s</td>
						<td>%s</td>
						<td>%s</td>
						<td>%s</td>
					</tr>
					`
				// <td style="padding-left: 3em;">%d</td>

				var priority string
				var status string
				for idx, t := range tasks {
					switch t.Priority {
					case 1:
						priority = "LOW"
					case 2:
						priority = "MEDIUM"
					case 3:
						priority = "HIGH"
					}
					switch t.Status {
					case 1:
						status = "NEW"
					case 2:
						status = "STARTED"
					case 3:
						status = "BLOCKED"
					case 4:
						status = "DONE"
					}
					if idx < (len(tasks) - 1) {
						w.Write([]byte(fmt.Sprintf(row, t.Name, priority, status, t.DueDate)))
						// w.Write([]byte(fmt.Sprintf(row, t.Name, priority, status, t.DueDate, t.ID)))
					} else {
						var otherCursorValueStr string
						switch otherCursorColumn {
						case "priority":
							otherCursorValueStr = strconv.Itoa(int(t.Priority))
						case "status":
							otherCursorValueStr = strconv.Itoa(int(t.Status))
						case "due_date":
							otherCursorValueStr = t.DueDate
						default:
							otherCursorValueStr = t.Name
						}
						w.Write([]byte(fmt.Sprintf(`
						<tr
								hx-get="/tasks"
								hx-include="input, select"
								hx-vals='{"idCursorValue": "%d", "otherCursorValue": "%s"}'
								hx-trigger="revealed"
								hx-swap="afterend"
								hx-indicator="#table_spinner"
						>
							<td>%s</td>
							<td>%s</td>
							<td>%s</td>
							<td>%s</td>
						</tr>
					`, t.ID, otherCursorValueStr, t.Name, priority, status, t.DueDate)))
						// <td style="padding-left: 3em;">%d</td>
						// `, t.ID, otherCursorValueStr, t.Name, priority, status, t.DueDate, t.ID)))
					}
				}
			}
		})

		http.HandleFunc("/task-count", func(w http.ResponseWriter, r *http.Request) {
			var searchPartMap url.Values = r.URL.Query()
			taskCount := countTasks(db,
				searchPartMap.Get("name"),
				searchPartMap.Get("nameSearchType"),
				convertQueryParamToUint8(searchPartMap, "priority"),
				convertQueryParamToUint8(searchPartMap, "status"),
				searchPartMap.Get("dueDate"),
			)
			var taskCountMsg string
			if taskCount == "1" {
				taskCountMsg = taskCount + " result"
			} else {
				taskCountMsg = taskCount + " results"
			}
			w.Write([]byte(taskCountMsg))
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

func fetchNext25Tasks(db *sql.DB, name string, nameSearchType string, priority uint8, status uint8, dueDate string,
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

	var index string
	switch otherCursorColumn {
	case "status":
		index = "tasks_status_id_idx"
	case "priority":
		index = "tasks_priority_id_idx"
	case "due_date":
		index = "tasks_due_date_id_idx"
	default:
		index = "tasks_name_id_idx"
	}

	sql := fmt.Sprintf(`SELECT id, name, priority, status, due_date FROM tasks
	INDEXED BY %s
	WHERE name LIKE ?
	AND priority %s
	AND status %s
	AND due_date LIKE ?
	AND (%s, id) %s (?, ?)
	ORDER BY %s %s, id %s
	LIMIT 25`, index, priorityClause, statusClause, otherCursorColumn, cursorOperator, otherCursorColumn, sortOrder, sortOrder)

	fmt.Println("\nSQL...fetch next 25 tasks")
	fmt.Println(sql + "\n")
	fmt.Println("NAME:", name)
	fmt.Println("DUE_DATE:", dueDate)
	fmt.Println("Other cursor value:", otherCursorValue)
	fmt.Println("ID cursor value:", idCursorValue)
	rows, err := db.Query(sql, name, dueDate, otherCursorValue, idCursorValue)
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

func fetchFirst25Tasks(db *sql.DB, name string, nameSearchType string, priority uint8, status uint8, dueDate string,
	sortColumn string, sortOrder string) []Task {
	switch nameSearchType {
	case "startsWith":
		name += "%"
	case "endsWith":
		name = "%" + name
	default:
		name = "%" + name + "%"
	}

	var priorityClause string
	switch priority {
	case 1, 2, 3:
		priorityClause = "= " + strconv.Itoa(int(priority))
	default:
		priorityClause = "IN (1, 2, 3)"
	}

	var statusClause string
	switch status {
	case 1, 2, 3, 4:
		statusClause = "= " + strconv.Itoa(int(status))
	default:
		statusClause = "IN (1, 2, 3, 4)"
	}

	dueDate += "%"

	if !slices.Contains([]string{"name", "priority", "status", "due_date"}, sortColumn) {
		sortColumn = "name"
	}

	if !slices.Contains([]string{"ASC", "DESC"}, strings.ToUpper(sortOrder)) {
		sortOrder = "ASC"
	}

	var index string
	switch sortColumn {
	case "status":
		index = "tasks_status_id_idx"
	case "priority":
		index = "tasks_priority_id_idx"
	case "due_date":
		index = "tasks_due_date_id_idx"
	default:
		index = "tasks_name_id_idx"
	}

	sql := fmt.Sprintf(`SELECT id, name, priority, status, due_date FROM tasks
	INDEXED BY %s
	WHERE name LIKE ?
	AND priority %s
	AND status %s
	AND due_date LIKE ?
	ORDER BY %s %s, id %s
	LIMIT 25`, index, priorityClause, statusClause, sortColumn, sortOrder, sortOrder)

	fmt.Println("\nSQL...fetch first 25 tasks")
	fmt.Println(sql + "\n")
	fmt.Println("NAME:", name)
	fmt.Println("DUE_DATE:", dueDate)

	var tasks []Task
	rows, err := db.Query(sql, name, dueDate)
	if err != nil {
		panic(err)
	} else {
		var task Task
		for rows.Next() {
			err := rows.Scan(&task.ID, &task.Name, &task.Priority, &task.Status, &task.DueDate)
			if err != nil {
				panic(err)
			} else {
				tasks = append(tasks, task)
			}
		}
	}
	return tasks
}

func countTasks(db *sql.DB, name string, nameSearchType string, priority uint8, status uint8, dueDate string) string {
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

	sql := fmt.Sprintf(`SELECT FORMAT('%%,d', COUNT(*)) FROM tasks
	WHERE name LIKE ?
	AND priority %s
	AND status %s
	AND due_date LIKE ?`, priorityClause, statusClause)

	fmt.Println("\nSQL...count tasks")
	fmt.Println(sql + "\n")
	fmt.Println("NAME:", name)
	fmt.Println("DUE_DATE:", dueDate)

	var count string
	err := db.QueryRow(sql, name, dueDate).Scan(&count)
	if err != nil {
		panic(err)
	}
	return count
}

type Task struct {
	ID       uint32
	Name     string
	Priority uint8
	Status   uint8
	DueDate  string // convert to time.Time objects only when needed
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
	OtherCursorValueStr     string
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
