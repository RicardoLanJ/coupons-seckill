package main

import (
	"github.com/gin-gonic/gin"
	"net/http"
	"database/sql"
	_ "github.com/go-sql-driver/mysql"
	"fmt"
	"github.com/sirupsen/logrus"
	"os"
)

type User struct {
	Id 		   int `json:"-" orm:"id"`
    UserName   string  `json:"username" orm:"username"`
    PassWord   string  `json:"password" orm:"password"` 
    Kind       int     `json:"kind,omitempty" orm:"kind"`  
}

var db *sql.DB
var log = logrus.New()

func init() {
	db, _ = initConnPool()
}

func main() {
	fp := initlogger()
	defer fp.Close()

	router := gin.Default()

	router.POST("/api/users", registerUser)
	router.POST("/api/auth", Userlogin)


	router.Run()
}

func Userlogin(c *gin.Context) {
	var user User
	if err := c.BindJSON(&user); err != nil {
		log.Error(err)
		c.JSON(http.StatusOK, gin.H{"kind":"0","errMsg":"json format wrong!"})
		return
	}
	var dbpassword, dbkind string
	result := db.QueryRow("select password,kind from users where username=?", user.UserName)
	if err := result.Scan(&dbpassword, &dbkind); err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusOK, gin.H{"kind":"0","errMsg":"user not exist!"})
			return
		}
		log.Fatal(err)
		return
	}
	if dbpassword == user.PassWord {
		log.WithFields(logrus.Fields{
			"username": user.UserName,
			"kind" : dbkind,
		  }).Info("login success")
	} else {
		c.JSON(http.StatusOK, gin.H{"kind":"0","errMsg":"password not correct!"})
	}
}

func registerUser(c *gin.Context) {
	var user User
	var temp string
	if err := c.BindJSON(&user); err != nil {
		log.Error(err)
		c.JSON(http.StatusOK, gin.H{"kind":"0","errMsg":"json format wrong!"})
		return
	}
	err := db.QueryRow("SELECT username FROM users WHERE username=?", user.UserName).Scan(&temp)
	errMsg := ""
	switch {
		case err == sql.ErrNoRows:
			log.WithFields(logrus.Fields{
				"username": user.UserName,
				"kind" : user.Kind,
			  }).Info("create user account")
			insertUserScript := "INSERT INTO users(username, password, kind) values(?,?,?)"
			// stmt, err := db.Prepare(insertUserScript)
			// defer stmt.Close()
			// if err != nil {
			// 	log.Fatal(err)
			// }
			// _, err = stmt.Exec(user.UserName, user.PassWord, user.Kind)
			_, err := db.Exec(insertUserScript, user.UserName, user.PassWord, user.Kind)
			if err != nil {
				log.Fatal(err)
			}
		case err != nil:
			log.Fatal(err)
		default:
			log.WithFields(logrus.Fields{
				"username": user.UserName,
			  }).Info("User existed")
			errMsg = "User existed"
	}
	c.JSON(http.StatusOK, gin.H{"errMsg":errMsg})
}

func initConnPool() (*sql.DB, error) {
	linkParam := "root:admin@tcp(127.0.0.1:3306)/msxt?charset=utf8"
	maxIdleConns := 1000
	maxOpenConns := 2000
	db, err := sql.Open("mysql", linkParam) 
	if err != nil {
			return nil, fmt.Errorf("Failed to connect to log mysql: %s", err)
	}
	//ping the mysql
	err = db.Ping()
	if err != nil {
			return nil, fmt.Errorf("Failed to ping mysql: %s", err)
	}

	fmt.Printf("maxIdleConns:%d\n", maxIdleConns)
	db.SetMaxIdleConns(maxIdleConns)
	
	fmt.Printf("maxOpenConns:%d\n", maxOpenConns)
	db.SetMaxOpenConns(maxOpenConns)

	fmt.Printf("dbSetupSuccess!\n")
	return db, nil
}

func initlogger() (fp *os.File){
	file, err := os.OpenFile("logrus.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	//defer file.Close()
	if err == nil {
		log.Out = file
	} else {
		log.Info("Failed to log to file, using default stderr")
	}
	return file
}