package main

import (
	"database/sql"
	// "fmt"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

type User struct {
	ID       int    `json:"id"`
	Role     string `json:"role"`
	Code     string `json:"code"`
	Login    string `json:"login"`
	Password string `json:"password"`
}

type Project struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Codes_users string `json:"codes_users"`
	Tasks       []Task `json:"tasks"`
}

type Task struct {
	ID         int    `json:"id"`
	Name       string `json:"name"`
	Descr      string `json:"descr"`
	Date       string `json:"date"`
	Date_act   string `json:"date_act"`
	Empl_code  string `json:"empl_code"`
	Project_id int    `json:"projectId"`
	Status     string `json:"status"`
}

func main() {
	// Create a new router
	db, err := sqlx.Open("postgres", "host=localhost port=5433 user=postgres password=0921 dbname=postgres sslmode=disable")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	r := gin.Default()

	r.POST("/login", loginHandler(db))
	r.GET("/projects/:code", projectsHandler(db))
	r.GET("/tasks/:id", tasksHandler(db))

	r.Run()
}

func loginHandler(db *sqlx.DB) gin.HandlerFunc {
	return gin.HandlerFunc(func(c *gin.Context) {
		var user User
		if err := c.BindJSON(&user); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		row := db.QueryRow("SELECT id, role, code FROM users WHERE login = $1 AND password = $2", user.Login, user.Password)

		err := row.Scan(&user.ID, &user.Role, &user.Code)
		if err != nil {
			if err == sql.ErrNoRows {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid login or password"})
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			}
			return
		}

		c.JSON(http.StatusOK, gin.H{"user": user})
	})
}

func projectsHandler(db *sqlx.DB) gin.HandlerFunc {
	return gin.HandlerFunc(func(c *gin.Context) {
		code := c.Param("code")

		var projects []Project
		err := db.Select(&projects, "SELECT id, name FROM projects WHERE codes_users LIKE $1", "%"+code+"%")
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		for i := range projects {
			var tasks []Task
			err = db.Select(&tasks, "SELECT id, name, status FROM tasks WHERE project_id = $1 AND empl_code = $2", projects[i].ID, code)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			projects[i].Tasks = tasks
		}

		c.JSON(http.StatusOK, gin.H{"projects": projects})
	})
}

func tasksHandler(db *sqlx.DB) gin.HandlerFunc {
	return gin.HandlerFunc(func(c *gin.Context) {
		code := c.Param("id")

		var task Task
		err := db.Select(&task, "SELECT name, descr, date, date_act, status FROM projects WHERE id = $1", code)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"task": task})
	})
}
