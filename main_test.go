package main

import (
	"database/sql"
	"fmt"
	"math/rand"
	"strconv"
	"strings"

	"testing"
)

func TestFetchFirstOrNext25Tasks(t *testing.T) {
	if db, err := sql.Open("sqlite3", "./task.db"); err == nil {
		numTests := 20

		var whereClause strings.Builder
		whereClause.WriteString(" WHERE id IN (")
		for i := 0; i < numTests; i++ {
			if i < (numTests - 1) {
				whereClause.WriteString(strconv.Itoa(rand.Intn(1_000_000)+1) + ", ")
			} else {
				whereClause.WriteString(strconv.Itoa(rand.Intn(1_000_000)+1) + ")")
			}
		}

		rows, err := db.Query(fmt.Sprintf("SELECT id, name, priority, status, due_date FROM tasks %s", whereClause.String()))
		if err != nil {
			t.Errorf("SQL for selecting %d seed tasks failed", numTests)
		}

		var tasks []Task
		for rows.Next() {
			var task Task
			err = rows.Scan(&task.ID, &task.Name, &task.Priority, &task.Status, &task.DueDate)
			if err != nil {
				t.Errorf("Failed to map a row to a Task struct")
			}
			tasks = append(tasks, task)
		}

		for _, task := range tasks {
			// parallelization below doesn't work with SQLite because there's no connection pool?
			// t.Run(fmt.Sprintf("Task=%v", task), func(t *testing.T) {
			// 	task := task // capture loop variable to avoid race conditions; thus, each Goroutine gets a unique and right Task
			// t.Parallel()

			var nameSearchValue string
			var nameSearchType string
			var dueDateSearchValue string
			nameWordArr := strings.Split(task.Name, " ")
			nameAndDueDateSeachType := rand.Intn(3)
			switch nameAndDueDateSeachType {
			case 0:
				nameSearchValue = nameWordArr[0]
				nameSearchType = "startsWith"
				dueDateSearchValue = task.DueDate[0:4]
			case 1:
				nameSearchValue = nameWordArr[1]
				nameSearchType = "contains"
				dueDateSearchValue = task.DueDate[0:7]
			default:
				nameSearchValue = nameWordArr[len(nameWordArr)-1]
				nameSearchType = "endsWith"
				dueDateSearchValue = task.DueDate
			}

			priorityValue := task.Priority
			statusValue := task.Status
			sortOrder := "ASC"
			if rand.Intn(2) == 0 {
				priorityValue = 0
				statusValue = 0
				sortOrder = "DESC"
			}

			var sortColumn string
			var otherCursorValue string
			sortColAndOtherCursorSelector := rand.Intn(4)
			switch sortColAndOtherCursorSelector {
			case 0:
				sortColumn = "name"
				otherCursorValue = task.Name
			case 1:
				sortColumn = "priority"
				otherCursorValue = strconv.Itoa(int(task.Priority))
			case 2:
				sortColumn = "status"
				otherCursorValue = strconv.Itoa(int(task.Status))
			case 3:
				sortColumn = "due_date"
				otherCursorValue = task.DueDate
			}

			// added to randomly test fetchingFirst25Tasks
			isFetchingFirst25Tasks := rand.Intn(2) == 1
			if isFetchingFirst25Tasks {
				task.ID = 0
				otherCursorValue = ""
			}

			tasks := fetchNext25Tasks(db, nameSearchValue, nameSearchType, priorityValue, statusValue, dueDateSearchValue, uint32(task.ID),
				sortColumn, otherCursorValue, sortOrder)

			for i, task := range tasks {
				switch nameAndDueDateSeachType {
				case 0:
					firstWordInName := task.Name[0:strings.Index(task.Name, " ")]
					if strings.ToLower(firstWordInName) != strings.ToLower(nameSearchValue) { // SQLite's LIKE operator is case insensitive
						t.Errorf("Name='%s' doesn't start with '%s'", task.Name, nameSearchValue)
					}
					if dueDateSearchValue != task.DueDate[0:4] {
						t.Errorf("DueDate='%s' doesn't start with '%s'", dueDateSearchValue, task.DueDate[0:4])
					}
				case 1:
					if !strings.Contains(strings.ToLower(task.Name), strings.ToLower(nameSearchValue)) {
						t.Errorf("Name='%s' doesn't contain '%s'", task.Name, nameSearchValue)
					}
					if dueDateSearchValue != task.DueDate[0:7] {
						t.Errorf("DueDate='%s' doesn't start with '%s'", dueDateSearchValue, task.DueDate[0:7])
					}
				default:
					lastWordInName := task.Name[strings.LastIndex(task.Name, " ")+1 : len(task.Name)]
					if strings.ToLower(lastWordInName) != strings.ToLower(nameSearchValue) {
						t.Errorf("Name='%s' doesn't end with '%s'", task.Name, nameSearchValue)
					}
					if dueDateSearchValue != task.DueDate {
						t.Errorf("DueDate='%s' isn't '%s'", dueDateSearchValue, task.DueDate)
					}
				}

				if priorityValue != 0 && statusValue != 0 {
					if task.Status != statusValue {
						t.Errorf("Status must be %d", statusValue)
					}
					if task.Priority != priorityValue {
						t.Errorf("Status must be %d", priorityValue)
					}
				}

				if i < len(tasks)-1 {
					nextTask := tasks[i+1]
					switch sortColAndOtherCursorSelector {
					case 0:
						if sortOrder == "ASC" {
							if nextTask.Name < task.Name || (nextTask.Name == task.Name && nextTask.ID < task.ID) {
								t.Errorf("Name sort ASC error: %v should come before %v", task, nextTask)
							}
						} else {
							if nextTask.Name > task.Name || (nextTask.Name == task.Name && nextTask.ID > task.ID) {
								t.Errorf("Name sort DESC error: %v should come before %v", nextTask, task)
							}
						}
					case 1:
						if sortOrder == "ASC" {
							if nextTask.Priority < task.Priority || (nextTask.Priority == task.Priority && nextTask.ID < task.ID) {
								t.Errorf("Priority sort ASC error: %v should come before %v", task, nextTask)
							}
						} else {
							if nextTask.Priority > task.Priority || (nextTask.Priority == task.Priority && nextTask.ID > task.ID) {
								t.Errorf("Priority sort DESC error: %v should come before %v", nextTask, task)
							}
						}
					case 2:
						if sortOrder == "ASC" {
							if nextTask.Status < task.Status || (nextTask.Status == task.Status && nextTask.ID < task.ID) {
								t.Errorf("Status sort ASC error: %v should come before %v", task, nextTask)
							}
						} else {
							if nextTask.Status > task.Status || (nextTask.Status == task.Status && nextTask.ID > task.ID) {
								t.Errorf("Status sort DESC error: %v should come before %v", nextTask, task)
							}
						}
					case 3:
						if sortOrder == "ASC" {
							if nextTask.DueDate < task.DueDate || (nextTask.DueDate == task.DueDate && nextTask.ID < task.ID) {
								t.Errorf("DueDate sort ASC error: %v should come before %v", task, nextTask)
							}
						} else {
							if nextTask.DueDate > task.DueDate || (nextTask.DueDate == task.DueDate && nextTask.ID > task.ID) {
								t.Errorf("DueDate sort DESC error: %v should come before %v", nextTask, task)
							}
						}
					}
				}
			}

			numTasksExpected := 25
			if len(tasks) > numTasksExpected {
				t.Errorf("taskSearch shouldn't retrieve more than %d rows", numTasksExpected)
			}
			// })
		}
		defer db.Close()
	} else {
		panic(err)
	}
}

func TestFetchNext25Tasks(t *testing.T) {
	db, err := sql.Open("sqlite3", "./task.db")
	if err != nil {
		panic(err)
	} else {
		nameSearchValue := "a"
		nameSearchType := "contains"
		var priority uint8 = 1
		var status uint8 = 1
		dueDate := "2025"
		var idCursorValue uint32 = 431409
		otherCursorColumn := "name"
		otherCursorValue := "Clean up codebase, Prepare presentation, Complete project report, Conduct performance review, Organize team meeting"
		sortOrder := "ASC"
		tasks := fetchNext25Tasks(db, nameSearchValue, nameSearchType, priority, status, dueDate, idCursorValue, otherCursorColumn,
			otherCursorValue, sortOrder)

		for i, task := range tasks {
			if !strings.Contains(task.Name, nameSearchValue) {
				t.Errorf("Name must contain '%s'", nameSearchValue)
			}
			if task.Status != status {
				t.Errorf("Status must be %d", status)
			}
			if task.Priority != priority {
				t.Errorf("Status must be %d", priority)
			}
			if !strings.Contains(task.DueDate, dueDate) {
				t.Errorf("Due date must be in %s", dueDate)
			}

			if i != 0 {
				prevTask := tasks[i-1]
				if task.ID == prevTask.ID {
					if task.ID < prevTask.ID {
						t.Errorf("Sort ordering not properly implemented; '%d' should come after '%d'", task.ID, prevTask.ID)
						t.Errorf("Sort ordering not properly implemented; '%v' should come after '%v'", task, prevTask)
					}
				}
				if task.Name < prevTask.Name {
					t.Errorf("Sort ordering not properly implemented; '%s' should come after '%s'", task.Name, prevTask.Name)
				}
			}
		}

		numTasksExpected := 25
		if len(tasks) != numTasksExpected {
			t.Errorf("taskSearch should retrieve %d rows", numTasksExpected)
		}
	}
	defer db.Close()
}

func TestCountTasks(t *testing.T) {
	db, err := sql.Open("sqlite3", "./task.db")
	if err != nil {
		panic(err)
	} else {
		taskCount := countTasks(db, "", "", 0, 0, "")
		if taskCount != "1,000,000" {
			t.Errorf("Search without values should yield 1,000,000 tasks; instead if was " + taskCount)
		}
		taskCount = countTasks(db, "A", "startsWith", 0, 0, "")
		if taskCount != "49,648" {
			t.Errorf("Search without values should yield 49,648 tasks; instead if was " + taskCount)
		}
		taskCount = countTasks(db, "E", "contains", 1, 2, "2025")
		if taskCount != "7,476" {
			t.Errorf("Search without values should yield 7,476 tasks; instead it was " + taskCount)
		}
		defer db.Close()
	}

}
