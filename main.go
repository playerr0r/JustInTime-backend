package main

import (
	"database/sql"

	"encoding/base64"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

type User struct {
	ID           int           `json:"id"`
	Name         string        `json:"name"`
	Role         string        `json:"role"`
	Code         string        `json:"code"`
	Login        string        `json:"login"`
	Password     string        `json:"password"`
	Projects_ids pq.Int64Array `json:"projects_ids"`
	Avatar       []byte        `json:"avatar"`
}

type Project struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Tasks []Task `json:"tasks"`
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

	r.GET("/projects/", projectsHandler(db))
	r.GET("/projects/:id/tasks", projectTasksHandler(db))

	r.GET("/tasks/:id", tasksHandler(db))

	r.GET("/profile/:id", profileHandler(db))

	r.POST("/profile/:id/update_avatar", profileUpdateAvatarHandler(db))

	r.Run()
}

// /login
func loginHandler(db *sqlx.DB) gin.HandlerFunc {
	return gin.HandlerFunc(func(c *gin.Context) {
		var user User
		if err := c.BindJSON(&user); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		row := db.QueryRow("SELECT id, name, role, code, projects_ids, avatar FROM users WHERE login = $1 AND password = $2", user.Login, user.Password)

		err := row.Scan(&user.ID, &user.Name, &user.Role, &user.Code, &user.Projects_ids, &user.Avatar)
		if err != nil {
			if err == sql.ErrNoRows {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid login or password"})
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			}
			return
		}

		user.Avatar = []byte(base64.StdEncoding.EncodeToString(user.Avatar))
		c.JSON(http.StatusOK, gin.H{"user": user})
	})
}

// /projects/?ids=0,1...
func projectsHandler(db *sqlx.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		idsParam := c.DefaultQuery("ids", "")
		idsStr := strings.Split(idsParam, ",")

		var ids []int
		for _, idStr := range idsStr {
			id, err := strconv.Atoi(idStr)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
				return
			}
			ids = append(ids, id)
		}

		projects := make(map[int]string)
		for _, id := range ids {
			var project_name string
			err := db.Get(&project_name, "SELECT name FROM projects WHERE id = $1", id)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			projects[id] = project_name
		}

		c.JSON(http.StatusOK, gin.H{"projects": projects})
	}
}

// /projects/:id/tasks
func projectTasksHandler(db *sqlx.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		projectID := c.Param("id")

		var tasks []Task
		err := db.Select(&tasks, `SELECT * FROM tasks WHERE project_id = $1`, projectID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"tasks": tasks})
	}
}

// /tasks/:id
func tasksHandler(db *sqlx.DB) gin.HandlerFunc {
	return gin.HandlerFunc(func(c *gin.Context) {
		id := c.Param("id")

		var task Task
		err := db.Get(&task, "SELECT * FROM tasks WHERE id = $1", id)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"task": task})
	})
}

// /profile/:id
func profileHandler(db *sqlx.DB) gin.HandlerFunc {
	return gin.HandlerFunc(func(c *gin.Context) {
		id := c.Param("id")

		var user User
		user.Login = ""
		user.Password = ""
		user.ID, _ = strconv.Atoi(id)

		err := db.Get(&user, "SELECT name, role, code, projects_ids, avatar FROM users WHERE id = $1", id)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// user.Avatar = []byte(base64.StdEncoding.EncodeToString(user.Avatar))
		c.JSON(http.StatusOK, gin.H{"user": user})
	})
}

// Define a struct to hold the incoming JSON data
type AvatarData struct {
	Avatar string `json:"avatar"`
}

// /profile/:id/update_avatar/:avatar
func profileUpdateAvatarHandler(db *sqlx.DB) gin.HandlerFunc {
	return gin.HandlerFunc(func(c *gin.Context) {
		id := c.Param("id")

		var jsonData AvatarData
		if err := c.BindJSON(&jsonData); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		avatarDecoded, err := base64.StdEncoding.DecodeString(jsonData.Avatar)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		fmt.Println(hex.EncodeToString(avatarDecoded[:100]))

		_, err = db.Exec("UPDATE users SET avatar = $1 WHERE id = $2", avatarDecoded, id)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Avatar updated"})
	})
}