package main

import (
	"database/sql"
	"strings"

	"testing"
)

func TestTaskSearch(t *testing.T) {
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
		tasks := taskSearch(db, nameSearchValue, nameSearchType, priority, status, dueDate, idCursorValue, otherCursorColumn,
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

		// TODO: when fuzz testing: if len(tasks) > maxNumTasksExpected { ... }
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
		if taskCount != 1_000_000 {
			t.Errorf("Search without values should yield 1,000,000 tasks")
		}
		taskCount = countTasks(db, "A", "startsWith", 0, 0, "")
		if taskCount != 49_648 {
			t.Errorf("Search without values should yield 49,648 tasks")
		}
		taskCount = countTasks(db, "E", "contains", 1, 2, "2025")
		if taskCount != 7_476 {
			t.Errorf("Search without values should yield 7_476 tasks")
		}
		defer db.Close()
	}

	// func FuzzTaskSearch(f *testing.F) {
	// TODO: complete function
	// }

}
