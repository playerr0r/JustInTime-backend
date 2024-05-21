package main

import (
	"database/sql"
	"fmt"
	"os"

	"encoding/base64"
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
	ID         int            `json:"id"`
	Name       string         `json:"name"`
	Descr      sql.NullString `json:"descr"`
	Date       string         `json:"date"`
	Date_act   sql.NullString `json:"date_act"`
	Empl_id    sql.NullString `json:"empl_id"`
	Avatar     []byte         `json:"avatar"`
	Project_id int            `json:"projectId"`
	Status     string         `json:"status"`
	Priority   sql.NullString `json:"priority"`
}

type TaskResponse struct {
	ID         int    `json:"id"`
	Name       string `json:"name"`
	Descr      string `json:"descr"`
	Date       string `json:"date"`
	Date_act   string `json:"date_act"`
	Empl_id    string `json:"empl_id"`
	Avatar     []byte `json:"avatar"`
	Project_id int    `json:"projectId"`
	Status     string `json:"status"`
	Priority   string `json:"priority"`
}

func main() {
	// Get environment variables
	databaseHost := os.Getenv("DATABASE_HOST")
	databaseUser := os.Getenv("DATABASE_USER")
	databasePassword := os.Getenv("DATABASE_PASSWORD")
	databasePort := "5432"
	databaseName := os.Getenv("DATABASE_NAME")

	// databaseHost := "localhost"
	// databaseUser := "postgres"
	// databasePort := "5433"
	// databasePassword := "0921"
	// databaseName := "postgres"

	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable", databaseHost, databasePort, databaseUser, databasePassword, databaseName)

	// Create a new router
	db, err := sqlx.Connect("postgres", dsn)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// gin.SetMode(gin.ReleaseMode)

	r := gin.Default()

	r.GET("/", startPageHandler())

	// Группировка маршрутов для регистрации и логина
	authRoutes := r.Group("/auth")
	{
		authRoutes.POST("/login", loginHandler(db))
		authRoutes.POST("/register", registerHandler(db))
		authRoutes.GET("/register/check/:login", checkLoginHandler(db))
	}

	// Группировка маршрутов для проектов
	projectRoutes := r.Group("/projects")
	{
		projectRoutes.GET("/", projectsHandler(db))
		projectRoutes.GET("/:id/tasks", projectTasksHandler(db))
		projectRoutes.DELETE("/:id", projectDeleteHandler(db))
		projectRoutes.POST("/new", projectNewHandler(db))
		projectRoutes.POST("/:id/column", projectNewColumnHandler(db))
		projectRoutes.DELETE("/:id/column", projectDeleteColumnHandler(db))
		projectRoutes.POST("/:id/column/update", projectUpdateColumnHandler(db))
	}

	// Группировка маршрутов для задач
	taskRoutes := r.Group("/tasks")
	{
		taskRoutes.GET("/:id", tasksHandler(db))
		taskRoutes.DELETE("/:id", taskDeleteHandler(db))
		taskRoutes.POST("/:id/updateStatus", taskStatusUpdateHandler(db))
		taskRoutes.POST("/:id/assign/", taskAssignHandler(db))
		taskRoutes.POST("/new", taskNewHandler(db))
	}

	// Профиль пользователя
	profileRoutes := r.Group("/profile")
	{
		profileRoutes.GET("/:id", profileHandler(db))
		profileRoutes.POST("/:id/updateAvatar", profileUpdateAvatarHandler(db))
		profileRoutes.POST("/:id/addProject", profileAddProjectHandler(db))
		profileRoutes.DELETE("/:id", profileRemoveProjectHandler(db))
	}

	r.Run()
}

func startPageHandler() gin.HandlerFunc {
	return gin.HandlerFunc(func(c *gin.Context) {
		fmt.Print("Service is live")
	})
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

// /register
func registerHandler(db *sqlx.DB) gin.HandlerFunc {
	return gin.HandlerFunc(func(c *gin.Context) {
		var user User
		if err := c.BindJSON(&user); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		_, err := db.Exec("INSERT INTO users (name, role, code, login, password) VALUES ($1, $2, $3, $4, $5)", user.Name, user.Role, user.Code, user.Login, user.Password)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "User registered"})
	})
}

// /register/check/:login
func checkLoginHandler(db *sqlx.DB) gin.HandlerFunc {
	return gin.HandlerFunc(func(c *gin.Context) {
		login := c.Param("login")

		var count int
		err := db.Get(&count, "SELECT COUNT(*) FROM users WHERE login = $1", login)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		if count > 0 {
			c.JSON(http.StatusOK, gin.H{"message": "Login exists"})
		} else {
			c.JSON(http.StatusOK, gin.H{"message": "Login is free"})
		}
	})
}

// /projects/?ids=
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

func nullStringToString(ns sql.NullString) string {
	if ns.Valid {
		return strings.Trim(ns.String, "{}")
	}
	return ""
}

// /projects/:id/tasks
func projectTasksHandler(db *sqlx.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		projectID := c.Param("id")

		var tasks []Task
		err := db.Select(&tasks, `SELECT tasks.id, tasks.name, tasks.descr, tasks.date, tasks.date_act, tasks.empl_id, users.avatar, tasks.project_id, tasks.status, tasks.priority from tasks left join users on tasks.empl_id = users.id WHERE project_id = $1`, projectID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		var columns []string
		err = db.Select(&columns, "SELECT columns_ FROM projects WHERE id = $1", projectID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		var tasksResponse []TaskResponse
		for _, task := range tasks {
			taskResponse := TaskResponse{
				ID:         task.ID,
				Name:       task.Name,
				Descr:      nullStringToString(task.Descr),
				Date:       task.Date,
				Date_act:   nullStringToString(task.Date_act),
				Empl_id:    nullStringToString(task.Empl_id),
				Avatar:     task.Avatar,
				Project_id: task.Project_id,
				Status:     task.Status,
				Priority:   nullStringToString(task.Priority),
			}
			tasksResponse = append(tasksResponse, taskResponse)
		}

		c.JSON(http.StatusOK, gin.H{"columns": columns, "tasks": tasksResponse})
	}
}

// /projects/:id DELETE
func projectDeleteHandler(db *sqlx.DB) gin.HandlerFunc {
	return gin.HandlerFunc(func(c *gin.Context) {
		id := c.Param("id")

		_, err := db.Exec("DELETE FROM projects WHERE id = $1", id)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		_, err = db.Exec("DELETE FROM tasks WHERE project_id = $1", id)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		_, err = db.Exec("UPDATE users SET projects_ids = array_remove(projects_ids, $1)", id)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Project" + id + " deleted"})
	})
}

// /projects/new
func projectNewHandler(db *sqlx.DB) gin.HandlerFunc {
	return gin.HandlerFunc(func(c *gin.Context) {
		var project Project
		if err := c.BindJSON(&project); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		_, err := db.Exec("INSERT INTO projects (name) VALUES ($1)", project.Name)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Project " + project.Name + " added"})
	})
}

// create new column /projects/:id/column
func projectNewColumnHandler(db *sqlx.DB) gin.HandlerFunc {
	return gin.HandlerFunc(func(c *gin.Context) {
		id := c.Param("id")

		var name string
		if err := c.BindJSON(&name); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			fmt.Println("error: ", err.Error())
			return
		}

		_, err := db.Exec("UPDATE projects SET columns_ = array_append(columns_, $1) WHERE id = $2", name, id)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			fmt.Println("error: ", err.Error())
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Column + " + name + " added"})
	})
}

// delete column /projects/:id/column
func projectDeleteColumnHandler(db *sqlx.DB) gin.HandlerFunc {
	return gin.HandlerFunc(func(c *gin.Context) {
		id := c.Param("id")

		var name string
		if err := c.BindJSON(&name); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		_, err := db.Exec("UPDATE projects SET columns_ = array_remove(columns_, $1) WHERE id = $2", name, id)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		_, err = db.Exec("DELETE FROM tasks WHERE project_id = $1 AND status = $2", id, name)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Column + " + name + " deleted"})
	})
}

// update name of column /projects/:id/column/update
func projectUpdateColumnHandler(db *sqlx.DB) gin.HandlerFunc {
	return gin.HandlerFunc(func(c *gin.Context) {
		id := c.Param("id")

		var old_name string
		var new_name string
		if err := c.BindJSON(&gin.H{"old_name": &old_name, "new_name": &new_name}); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		_, err := db.Exec("UPDATE projects SET columns_ = array_replace(columns_, $1, $2) WHERE id = $3", old_name, new_name, id)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		_, err = db.Exec("UPDATE tasks SET status = $1 WHERE project_id = $2 AND status = $3", new_name, id, old_name)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Column" + old_name + " updated to " + new_name})
	})
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

		var taskResponse TaskResponse
		taskResponse.ID = task.ID
		taskResponse.Name = task.Name
		taskResponse.Descr = nullStringToString(task.Descr)
		taskResponse.Date = task.Date
		taskResponse.Date_act = nullStringToString(task.Date_act)
		taskResponse.Empl_id = nullStringToString(task.Empl_id)
		taskResponse.Project_id = task.Project_id
		taskResponse.Status = task.Status
		taskResponse.Priority = nullStringToString(task.Priority)

		c.JSON(http.StatusOK, gin.H{"task": taskResponse})
	})
}

// /tasks/:id
func taskDeleteHandler(db *sqlx.DB) gin.HandlerFunc {
	return gin.HandlerFunc(func(c *gin.Context) {
		id := c.Param("id")

		_, err := db.Exec("DELETE FROM tasks WHERE id = $1", id)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Task deleted"})
	})
}

// /tasks/:id/updateStatus
func taskStatusUpdateHandler(db *sqlx.DB) gin.HandlerFunc {
	return gin.HandlerFunc(func(c *gin.Context) {
		id := c.Param("id")

		var task Task
		if err := c.BindJSON(&task); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		_, err := db.Exec("UPDATE tasks SET status = $1 WHERE id = $2", task.Status, id)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Task status updated"})
	})
}

// /tasks/:id/assign/?empl_id=
func taskAssignHandler(db *sqlx.DB) gin.HandlerFunc {
	return gin.HandlerFunc(func(c *gin.Context) {
		id := c.Param("id")
		empl_id := c.DefaultQuery("empl_id", "")

		_, err := db.Exec("UPDATE tasks SET empl_id = $1 WHERE id = $2", empl_id, id)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Task assigned"})
	})
}

// /tasks/new
func taskNewHandler(db *sqlx.DB) gin.HandlerFunc {
	return gin.HandlerFunc(func(c *gin.Context) {
		var task Task

		task.Date_act = sql.NullString{String: "", Valid: false}
		task.Empl_id = sql.NullString{String: "", Valid: false}
		task.Priority = sql.NullString{String: "", Valid: false}
		task.Descr = sql.NullString{String: "", Valid: false}

		if err := c.BindJSON(&task); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		_, err := db.Exec("INSERT INTO tasks (name, descr, date, date_act, empl_id, project_id, status, priority) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)",
			task.Name, task.Descr, task.Date, task.Date_act, task.Empl_id, task.Project_id, task.Status, task.Priority)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Task added"})
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

		_, err = db.Exec("UPDATE users SET avatar = $1 WHERE id = $2", avatarDecoded, id)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Avatar updated"})
	})
}

// /profile/:id/addProject/:project_id
func profileAddProjectHandler(db *sqlx.DB) gin.HandlerFunc {
	return gin.HandlerFunc(func(c *gin.Context) {
		id := c.Param("id")
		project_id := c.Param("project_id")

		var projects_ids pq.Int64Array
		err := db.Get(&projects_ids, "SELECT projects_ids FROM users WHERE id = $1", id)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		project_id_int, _ := strconv.Atoi(project_id)
		projects_ids = append(projects_ids, int64(project_id_int))

		_, err = db.Exec("UPDATE users SET projects_ids = $1 WHERE id = $2", projects_ids, id)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Project added to user"})
	})
}

// /profile/:id/removeProject/:project_id
func profileRemoveProjectHandler(db *sqlx.DB) gin.HandlerFunc {
	return gin.HandlerFunc(func(c *gin.Context) {
		id := c.Param("id")
		project_id := c.Param("project_id")

		_, err := db.Exec("UPDATE users SET projects_ids = array_remove(projects_ids, $1) WHERE id = $2", project_id, id)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Project removed from user"})
	})
}
