package main

import (
	"database/sql"
	"fmt"
	"os"

	"encoding/base64"
	"log"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

type User struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	Role     string `json:"role"`
	Login    string `json:"login"`
	Password string `json:"password"`
	Avatar   []byte `json:"avatar"`
	Status   string `json:"status"`
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
	Creator_id int            `json:"creator_id"`
}

type Grant struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Descr      string `json:"descr"`
	Num        int    `json:"num"`
	Project_id int    `json:"projectId"`
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
	Creator_id int    `json:"creator_id"`
	Files      []File `json:"files"`
}

type File struct {
	ID       int    `json:"id"`
	TaskID   int    `json:"task_id"`
	Name     string `json:"name"`
	FileUuid string `json:"file_uuid"`
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
		projectRoutes.GET("/:id/users", projectUsersHandler(db))
		projectRoutes.POST("/:id/addUser", projectAddUserHandler(db))
		projectRoutes.DELETE("/:id/removeUser", projectDeleteUserHandler(db))
		projectRoutes.POST("/:id/rename", projectRenameHandler(db))
		projectRoutes.GET("/:id/grants", projectGrantsHandler(db))
		projectRoutes.POST("/:id/addGrant", projectAddGrantHandler(db))
		projectRoutes.DELETE("/:id/removeGrant", projectDeleteGrantHandler(db))
		projectRoutes.POST("/:id/editGrant", projectEditGrantHandler(db))
		projectRoutes.GET("/:id/usersOnline", projectUsersOnlineHandler(db))
	}

	// Группировка маршрутов для задач
	taskRoutes := r.Group("/tasks")
	{
		taskRoutes.GET("/:id", tasksHandler(db))
		taskRoutes.DELETE("/:id", taskDeleteHandler(db))
		taskRoutes.POST("/:id/updateStatus", taskStatusUpdateHandler(db))
		taskRoutes.POST("/:id/assign/", taskAssignHandler(db))
		taskRoutes.POST("/new", taskNewHandler(db))
		taskRoutes.POST("/:id/updateInfo", taskInfoUpdateHandler(db))
		taskRoutes.POST("/:id/updatePriority", taskPriorityUpdateHandler(db))
		taskRoutes.POST("/:id/addFile", taskAddFileHandler(db))
	}

	// Профиль пользователя
	profileRoutes := r.Group("/profile")
	{
		profileRoutes.GET("/:id", profileHandler(db))
		profileRoutes.POST("/:id/updateAvatar", profileUpdateAvatarHandler(db))
		profileRoutes.POST("/:id/addProject", profileAddProjectHandler(db))
		profileRoutes.GET("/:id/projects", profileProjectsHandler(db))
		profileRoutes.DELETE("/:id", profileRemoveProjectHandler(db))
		profileRoutes.POST("/:id/updateOnlineStatus", profileUpdateOnlineStatusHandler(db))
		// uploadImageHandler())
	}

	r.Run()
}

func startPageHandler() gin.HandlerFunc {
	return gin.HandlerFunc(func(c *gin.Context) {
		fmt.Print("Service is live \n")
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

		row := db.QueryRow("SELECT id, name, role, avatar, status FROM users WHERE login = $1 AND password = $2", user.Login, user.Password)

		err := row.Scan(&user.ID, &user.Name, &user.Role, &user.Avatar, &user.Status)
		if err != nil {
			if err == sql.ErrNoRows {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid login or password"})
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			}
			return
		}

		user.Avatar = []byte(base64.StdEncoding.EncodeToString(user.Avatar))

		rows, err := db.Query("SELECT project_id FROM user_projects WHERE user_id = $1", user.ID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		defer rows.Close()

		var projectsIDs []int
		for rows.Next() {
			var id int
			if err := rows.Scan(&id); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			projectsIDs = append(projectsIDs, id)
		}

		c.JSON(http.StatusOK, gin.H{"user": user, "projects": projectsIDs})
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

		_, err := db.Exec("INSERT INTO users (name, role, login, password, status) VALUES ($1, $2, $3, $4, $5)", user.Name, user.Role, user.Login, user.Password, user.Status)
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
		err := db.Select(&tasks, `SELECT tasks.id, tasks.name, tasks.descr, tasks.date, tasks.date_act, tasks.empl_id, users.avatar, tasks.project_id, tasks.status, tasks.priority, tasks.creator_id from tasks left join users on tasks.empl_id = users.id WHERE project_id = $1`, projectID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			fmt.Println("error: ", err.Error())
			return
		}

		var columns []string
		err = db.Select(&columns, "SELECT columns_ FROM projects WHERE id = $1", projectID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			fmt.Println("error: ", err.Error())
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
				Creator_id: task.Creator_id,
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

		_, err = db.Exec("DELETE FROM user_projects WHERE project_id = $1", id)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Project" + id + " deleted"})
	})
}

type NewProject struct {
	Name   string   `json:"name"`
	Logins []string `json:"logins"`
}

// /projects/new
func projectNewHandler(db *sqlx.DB) gin.HandlerFunc {
	return gin.HandlerFunc(func(c *gin.Context) {
		var project NewProject
		if err := c.BindJSON(&project); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			fmt.Println("error: ", err.Error())
			return
		}

		row := db.QueryRow("select * from projects where name = $1", project.Name)
		var tempProject NewProject
		err := row.Scan(&tempProject)
		if err == sql.ErrNoRows {
			// Проект с таким именем не найден, продолжить создание
		} else if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			fmt.Println("error: ", err.Error())
			return
		} else {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Project with this name already exists"})
			return
		}

		_, err = db.Exec("INSERT INTO projects (name) VALUES ($1)", project.Name)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			fmt.Println("error: ", err.Error())
			return
		}

		var projectID int
		err = db.Get(&projectID, "SELECT id FROM projects WHERE name = $1", project.Name)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			fmt.Println("error: ", err.Error())
			return
		}

		for _, login := range project.Logins {
			var userID int
			err = db.Get(&userID, "SELECT id FROM users WHERE login = $1", login)
			if err != nil {
				if err == sql.ErrNoRows {
					// Если пользователь не найден, пропустить этот логин и перейти к следующему
					continue
				} else {
					c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
					fmt.Println("error: ", err.Error())
					return
				}
			}

			_, err = db.Exec("INSERT INTO user_projects (user_id, project_id) VALUES ($1, $2)", userID, projectID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				fmt.Println("error: ", err.Error())
				return
			}

		}

		c.JSON(http.StatusOK, gin.H{"message": "Project " + project.Name + " added"})
	})
}

type Column struct {
	Name string `json:"name"`
}

// create new column /projects/:id/column
func projectNewColumnHandler(db *sqlx.DB) gin.HandlerFunc {
	return gin.HandlerFunc(func(c *gin.Context) {
		id := c.Param("id")

		var column Column
		if err := c.BindJSON(&column); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			fmt.Println("error: ", err.Error())
			return
		}

		_, err := db.Exec("UPDATE projects SET columns_ = array_append(columns_, $1) WHERE id = $2", column.Name, id)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			fmt.Println("error: ", err.Error())
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Column + " + column.Name + " added"})
	})
}

// delete column /projects/:id/column
func projectDeleteColumnHandler(db *sqlx.DB) gin.HandlerFunc {
	return gin.HandlerFunc(func(c *gin.Context) {
		id := c.Param("id")

		var column Column
		if err := c.BindJSON(&column); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		_, err := db.Exec("UPDATE projects SET columns_ = array_remove(columns_, $1) WHERE id = $2", column.Name, id)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			fmt.Println("error: ", err.Error())
			return
		}

		_, err = db.Exec("DELETE FROM tasks WHERE project_id = $1 AND status = $2", id, column.Name)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			fmt.Println("error: ", err.Error())
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Column + " + column.Name + " deleted"})
	})
}

type ColumnUpdate struct {
	Old_name string `json:"old_name"`
	New_name string `json:"new_name"`
}

// update name of column /projects/:id/column/update
func projectUpdateColumnHandler(db *sqlx.DB) gin.HandlerFunc {
	return gin.HandlerFunc(func(c *gin.Context) {
		id := c.Param("id")

		var columnUpdate ColumnUpdate
		if err := c.BindJSON(&columnUpdate); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			fmt.Println("error: ", err.Error())
			return
		}

		_, err := db.Exec("UPDATE projects SET columns_ = array_replace(columns_, $1, $2) WHERE id = $3", columnUpdate.Old_name, columnUpdate.New_name, id)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			fmt.Println("error: ", err.Error())
			return
		}

		_, err = db.Exec("UPDATE tasks SET status = $1 WHERE project_id = $2 AND status = $3", columnUpdate.New_name, id, columnUpdate.Old_name)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			fmt.Println("error: ", err.Error())
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Column" + columnUpdate.Old_name + " updated to " + columnUpdate.New_name})
	})
}

// /projects/:id/users
func projectUsersHandler(db *sqlx.DB) gin.HandlerFunc {
	return gin.HandlerFunc(func(c *gin.Context) {
		id := c.Param("id")

		var users []User
		err := db.Select(&users, `SELECT users.id, users.name, users.role, users.avatar FROM users left join user_projects on users.id = user_projects.user_id WHERE user_projects.project_id = $1`, id)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"users": users})
	})
}

// /projects/:id/addUser
func projectAddUserHandler(db *sqlx.DB) gin.HandlerFunc {
	return gin.HandlerFunc(func(c *gin.Context) {
		var user_login User
		if err := c.BindJSON(&user_login); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		var userID int
		err := db.Get(&userID, "SELECT id FROM users WHERE login = $1", user_login.Login)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		idStr := c.Param("id")
		projectId, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid project ID"})
			return
		}

		_, err = db.Exec("INSERT INTO user_projects (user_id, project_id) VALUES ($1, $2)", userID, projectId)
		if err != nil {
			if pqErr, ok := err.(*pq.Error); ok {
				if pqErr.Code == "23505" {
					c.JSON(http.StatusBadRequest, gin.H{"error": "User already in project"})
					return
				}
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "User added to project"})
	})
}

// /projects/:id/removeUser
func projectDeleteUserHandler(db *sqlx.DB) gin.HandlerFunc {
	return gin.HandlerFunc(func(c *gin.Context) {
		var user_name User
		if err := c.BindJSON(&user_name); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		fmt.Println(user_name.Name)

		var userID int
		err := db.Get(&userID, "SELECT id FROM users WHERE name = $1", user_name.Name)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			fmt.Println("error: ", err.Error())
			return
		}

		idStr := c.Param("id")
		projectId, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid project ID"})
			fmt.Println("error: ", err.Error())
			return
		}

		_, err = db.Exec("DELETE FROM user_projects WHERE user_id = $1 AND project_id = $2", userID, projectId)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			fmt.Println("error: ", err.Error())
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "User removed from project"})
	})
}

// /projects/:id/rename
func projectRenameHandler(db *sqlx.DB) gin.HandlerFunc {
	return gin.HandlerFunc(func(c *gin.Context) {
		id := c.Param("id")

		var project NewProject
		if err := c.BindJSON(&project); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		_, err := db.Exec("UPDATE projects SET name = $1 WHERE id = $2", project.Name, id)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Project renamed"})
	})
}

// /projects/:id/grants
func projectGrantsHandler(db *sqlx.DB) gin.HandlerFunc {
	return gin.HandlerFunc(func(c *gin.Context) {
		id := c.Param("id")

		var grants []Grant
		err := db.Select(&grants, "SELECT * FROM grants WHERE project_id = $1", id)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"grants": grants})
	})
}

// /projects/:id/addGrant
func projectAddGrantHandler(db *sqlx.DB) gin.HandlerFunc {
	return gin.HandlerFunc(func(c *gin.Context) {
		id := c.Param("id")

		var grant Grant
		if err := c.BindJSON(&grant); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			fmt.Println("error: ", err.Error())
			return
		}

		_, err := db.Exec("INSERT INTO grants (name, descr, num, project_id) VALUES ($1, $2, $3, $4)", grant.Name, grant.Descr, grant.Num, id)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			fmt.Println("error: ", err.Error())
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Grant added"})
	})
}

// /projects/:id/removeGrant
func projectDeleteGrantHandler(db *sqlx.DB) gin.HandlerFunc {
	return gin.HandlerFunc(func(c *gin.Context) {
		id := c.Param("id")

		var grant Grant
		if err := c.BindJSON(&grant); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		_, err := db.Exec("DELETE FROM grants WHERE name = $1 AND project_id = $2", grant.Name, id)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Grant deleted"})
	})
}

// /projects/:id/editGrant
func projectEditGrantHandler(db *sqlx.DB) gin.HandlerFunc {
	return gin.HandlerFunc(func(c *gin.Context) {
		id := c.Param("id")

		var grant Grant
		if err := c.BindJSON(&grant); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			fmt.Println("error: ", err.Error())
			return
		}

		fmt.Println(grant.Num)

		_, err := db.Exec("UPDATE grants SET descr = $1, num = $2, name = $3 WHERE id = $4 AND project_id = $5", grant.Descr, grant.Num, grant.Name, grant.ID, id)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			fmt.Println("error: ", err.Error())
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Grant updated"})
	})
}

// /projects/:id/usersOnline
func projectUsersOnlineHandler(db *sqlx.DB) gin.HandlerFunc {
	return gin.HandlerFunc(func(c *gin.Context) {
		id := c.Param("id")

		var users []User
		err := db.Select(&users, `SELECT users.id, users.name, users.avatar FROM users left join user_projects on users.id = user_projects.user_id WHERE user_projects.project_id = $1 and users.status = 'online'`, id)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"users": users})
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
			fmt.Println("error: ", err.Error())
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
		taskResponse.Creator_id = task.Creator_id

		rows, err := db.Query("SELECT id, name, object_name FROM files WHERE task_id = $1", id)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			fmt.Println("error: ", err.Error())
			return
		}
		defer rows.Close()

		var files []File
		for rows.Next() {
			var file File
			if err := rows.Scan(&file.ID, &file.Name, &file.FileUuid); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				fmt.Println("error: ", err.Error())
				return
			}
			file.TaskID, _ = strconv.Atoi(id)
			files = append(files, file)
		}

		taskResponse.Files = files

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
			fmt.Println("error: ", err.Error())
			return
		}

		_, err := db.Exec("INSERT INTO tasks (name, descr, date, date_act, empl_id, project_id, status, priority, creator_id) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)",
			task.Name, task.Descr, task.Date, task.Date_act, task.Empl_id, task.Project_id, task.Status, task.Priority, task.Creator_id)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			fmt.Println("error: ", err.Error())
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Task added"})
	})
}

type TaskInfo struct {
	Name  string `json:"name"`
	Descr string `json:"descr"`
}

// /tasks/:id/updateInfo
func taskInfoUpdateHandler(db *sqlx.DB) gin.HandlerFunc {
	return gin.HandlerFunc(func(c *gin.Context) {
		id := c.Param("id")

		var task TaskInfo
		if err := c.BindJSON(&task); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		_, err := db.Exec("UPDATE tasks SET name = $1, descr = $2 WHERE id = $3",
			task.Name, task.Descr, id)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			fmt.Println("error: ", err.Error())
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Task info updated"})
	})
}

type TaskPriority struct {
	Priority string `json:"priority"`
}

// /tasks/:id/updatePriority
func taskPriorityUpdateHandler(db *sqlx.DB) gin.HandlerFunc {
	return gin.HandlerFunc(func(c *gin.Context) {
		id := c.Param("id")

		var priority TaskPriority
		if err := c.BindJSON(&priority); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			fmt.Println("error: ", err.Error())
			return
		}

		_, err := db.Exec("UPDATE tasks SET priority = $1 WHERE id = $2", priority.Priority, id)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			fmt.Println("error: ", err.Error())
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Task priority updated"})
	})
}

// /tasks/:id/addFile
func taskAddFileHandler(db *sqlx.DB) gin.HandlerFunc {
	return gin.HandlerFunc(func(c *gin.Context) {
		id := c.Param("id")

		file, header, err := c.Request.FormFile("file")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			fmt.Println("error: ", err.Error())
			return
		}
		defer file.Close()

		accessKey := os.Getenv("ACCESS_KEY")
		secretKey := os.Getenv("SECRET_KEY")
		bucketName := os.Getenv("BUCKET_NAME_FILES")
		regionName := os.Getenv("REGION_NAME")
		fileName := header.Filename
		fileExt := filepath.Ext(fileName)
		objectName := uuid.New().String() + fileExt

		sess, err := session.NewSession(&aws.Config{
			Region:      aws.String(regionName),
			Credentials: credentials.NewStaticCredentials(accessKey, secretKey, ""),
			Endpoint:    aws.String("https://storage.yandexcloud.net"),
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка создания сессии: " + err.Error()})
			return
		}

		uploader := s3manager.NewUploader(sess)

		_, err = uploader.Upload(&s3manager.UploadInput{
			Bucket: aws.String(bucketName),
			Key:    aws.String(objectName),
			Body:   file,
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			fmt.Println("error: ", err.Error())
			return
		}

		_, err = db.Exec("INSERT INTO files (task_id, object_name, name) VALUES ($1, $2, $3)", id, objectName, fileName)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "File added"})
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

		err := db.Get(&user, "SELECT name, role, avatar FROM users WHERE id = $1", id)
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

// /profile/:id/update_avatar
func profileUpdateAvatarHandler(db *sqlx.DB) gin.HandlerFunc {
	return gin.HandlerFunc(func(c *gin.Context) {
		// id := c.Param("id")
		file, header, err := c.Request.FormFile("image")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			fmt.Println("error: ", err.Error())
			return
		}
		defer file.Close()

		accessKey := os.Getenv("ACCESS_KEY")
		secretKey := os.Getenv("SECRET_KEY")
		bucketName := os.Getenv("BUCKET_NAME_AVATARS")
		regionName := os.Getenv("REGION_NAME")
		objectName := header.Filename // Используем имя файла из заголовка

		sess, err := session.NewSession(&aws.Config{
			Region:      aws.String(regionName),
			Credentials: credentials.NewStaticCredentials(accessKey, secretKey, ""),
			Endpoint:    aws.String("https://storage.yandexcloud.net"),
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка создания сессии: " + err.Error()})
			return
		}

		uploader := s3manager.NewUploader(sess)

		// Загрузка файла
		_, err = uploader.Upload(&s3manager.UploadInput{
			Bucket: aws.String(bucketName),
			Key:    aws.String(objectName),
			Body:   file,
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			fmt.Println("error: ", err.Error())
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("Файл %s успешно загружен в бакет %s", objectName, bucketName)})

		c.JSON(http.StatusOK, gin.H{"message": "Avatar updated"})
	})
}

// /profile/:id/addProject/:project_id
func profileAddProjectHandler(db *sqlx.DB) gin.HandlerFunc {
	return gin.HandlerFunc(func(c *gin.Context) {
		id := c.Param("id")
		project_id := c.Param("project_id")

		// INSERT INTO user_projects (user_id, project_id) VALUES ($1, $2)
		_, err := db.Exec("INSERT INTO user_projects (user_id, project_id) VALUES ($1, $2)", id, project_id)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	})
}

// /profile/:id/projects
func profileProjectsHandler(db *sqlx.DB) gin.HandlerFunc {
	return gin.HandlerFunc(func(c *gin.Context) {
		idStr := c.Param("id")
		id, err := strconv.Atoi(idStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
			fmt.Println("error: ", err.Error())
			return
		}

		rows, err := db.Query("SELECT projects.id, projects.name FROM projects JOIN user_projects ON projects.id = user_projects.project_id WHERE user_projects.user_id = $1", id)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			fmt.Println("error: ", err.Error())
			return
		}
		defer rows.Close()

		var projects []map[string]interface{}
		for rows.Next() {
			var id int
			var name string
			if err := rows.Scan(&id, &name); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			project := map[string]interface{}{
				"project_id":   id,
				"project_name": name,
			}
			projects = append(projects, project)
		}

		c.JSON(http.StatusOK, gin.H{"projects": projects})
	})
}

// /profile/:id/removeProject/:project_id
func profileRemoveProjectHandler(db *sqlx.DB) gin.HandlerFunc {
	return gin.HandlerFunc(func(c *gin.Context) {
		id := c.Param("id")
		project_id := c.Param("project_id")

		_, err := db.Exec("DELETE FROM user_projects WHERE user_id = $1 AND project_id = $2", id, project_id)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Project removed from user"})
	})
}

// /profile/:id/updateOnlineStatus
func profileUpdateOnlineStatusHandler(db *sqlx.DB) gin.HandlerFunc {
	return gin.HandlerFunc(func(c *gin.Context) {
		id := c.Param("id")

		var user User
		if err := c.BindJSON(&user); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		_, err := db.Exec("UPDATE users SET status = $1 WHERE id = $2", user.Status, id)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Online status updated"})
	})
}

func uploadImageHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		fmt.Println("uploadImageHandler")
		file, header, err := c.Request.FormFile("image")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			fmt.Println("error: ", err.Error())
			return
		}
		defer file.Close()

		// Замените эти значения на свои
		accessKey := "test"
		secretKey := "test"
		bucketName := "test"
		regionName := "ru-central1"
		objectName := header.Filename // Используем имя файла из заголовка

		sess, err := session.NewSession(&aws.Config{
			Region:      aws.String(regionName),
			Credentials: credentials.NewStaticCredentials(accessKey, secretKey, ""),
			Endpoint:    aws.String("https://storage.yandexcloud.net"),
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка создания сессии: " + err.Error()})
			return
		}

		uploader := s3manager.NewUploader(sess)

		// Загрузка файла
		_, err = uploader.Upload(&s3manager.UploadInput{
			Bucket: aws.String(bucketName),
			Key:    aws.String(objectName),
			Body:   file,
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			fmt.Println("error: ", err.Error())
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("Файл %s успешно загружен в бакет %s", objectName, bucketName)})
	}
}
