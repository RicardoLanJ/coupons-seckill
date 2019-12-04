package main

import (
	"github.com/gin-gonic/gin"
	"net/http"
	"database/sql"
	_ "github.com/go-sql-driver/mysql"
	"fmt"
	"github.com/sirupsen/logrus"
	"os"
	"github.com/dgrijalva/jwt-go"
	"github.com/go-redis/redis"
	"strconv"
	"errors"
)

type User struct {
	Id 		   int     `json:"-" orm:"id"`
    UserName   string  `json:"username" orm:"username"`
    PassWord   string  `json:"password" orm:"password"` 
    Kind       int     `json:"kind,omitempty" orm:"kind"`  
}

type Coupon struct {
	Name string `json:"name"`
	Amount int `json:"amount"`
	Left int `json:"left,omitempty"`
	Description string `json:"description"`
	Stock int `json:"stock"`
}


type myClaims struct {
	UserName   string   `json:"username"`
	Kind       int     `json:"kind"`
    jwt.StandardClaims
}

var RedisClient *redis.Client
var db *sql.DB
var log = logrus.New()

const (
	scretKey = "konosuba"
)

func init() {
	db, _ = initConnPool()
	RedisClient = redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		Password: "",
		DB: 0,
	})
	_, err := RedisClient.Ping().Result()
	if err != nil {
		log.Fatal("connnect redis fail")
	}
}

func main() {
	fp := initlogger()
	defer fp.Close()

	router := gin.Default()  //debug mode!!! need to modify
	// gin.SetMode(gin.ReleaseMode)
	// router := gin.New()

	router.POST("/api/users", registerUser)
	router.POST("/api/auth", Userlogin)
	router.POST("/api/users/:username/coupons", Auth(), addCoupons)
	router.GET("/api/users/:username/coupons", Auth(), getCoupons)

	router.PATCH("/api/users/:username/coupons/:name", Auth(), secKillCoupons)

	router.Run()
}

func optimisticLockSK(key, username string) error {
	txf := func(tx *redis.Tx) error {
		// get current value or zero
		n, err := tx.Get(key).Int()
		if err != nil && err != redis.Nil {
			return err
		}
		isExist, err := tx.SIsMember(username, key).Result()
		if err != nil && err != redis.Nil {
			return err
		}
		if n == 0  {
			return errors.New("snapped up")
		}
		if isExist {
			return errors.New("can only get one")
		}
		// if n == 0?
		// actual opperation (local in optimistic lock)
		n--

		// runs only if the watched keys remain unchanged
		_, err = tx.Pipelined(func(pipe redis.Pipeliner) error {
			// pipe handles the error case
			pipe.Set(key, n, 0)
			pipe.SAdd(username, key)
			return nil
		})
		return err
	}

	for {
		err := RedisClient.Watch(txf, key)
		if err != redis.TxFailedErr {
			return err
		}
		// optimistic lock lost
	}
	//return errors.New("increment reached maximum number of retries")
}

func secKillCoupons(c *gin.Context) {
	//Busername := c.Param("username") // 没批用
	couponname := c.Param("name")
	username := c.MustGet("username").(string)
	kind := c.MustGet("kind").(int)
	if kind == 1 {
		c.JSON(http.StatusUnauthorized, gin.H{"errMsg":"商家抢你妈呢"})
		return
	}
	left, err := RedisClient.Get(couponname).Int()
	if err != nil {
		log.Fatal(err)
	}
	if left == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"errMsg":"snapped up"})
		return
	} else {
		if err = optimisticLockSK(couponname, username); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"errMsg":err.Error()})
		} else {
			c.JSON(http.StatusOK, gin.H{"errMsg":""})
		}
	}
}

//type Pages []Coupon
func getCoupons(c *gin.Context) {
	//page := c.Query("page")
	Pusername := c.Param("username")
	username := c.MustGet("username").(string)
	kind := c.MustGet("kind").(int)
	if kind == 0 && Pusername != username {
		c.JSON(http.StatusUnauthorized, gin.H{"errMsg":"you have no authority", "data":make([]Coupon, 0)})
		return
	}
	all, err := RedisClient.SMembers(username).Result()
	if err != nil {
        log.Fatal(err)
	}
	//fmt.Println(all, page)
	var allCoupon = []Coupon{}
	for _, v := range all {
		info, err := RedisClient.HGetAll(v + "-info").Result()
		//fmt.Println(v)
		if err != nil {
			log.Error("hget wrong")
			log.Fatal(err)
		}
		left, err := RedisClient.Get(v).Result()
		if err != nil {
			log.Fatal(err)
		}
		amount, _ := strconv.Atoi(info["amount"])
		intleft, _ := strconv.Atoi(left)
		if kind == 0 {
			amount = 1
			intleft = 1
		}
		stock, _ := strconv.Atoi(info["stock"])
		allCoupon = append(allCoupon, Coupon{
			v, //coupon name
			amount,
			intleft,
			info["description"],
			stock,
		})
	}
	c.JSON(http.StatusOK, gin.H{"errMsg":"", "data":allCoupon})
}

func addCoupons(c *gin.Context) {
	Pusername := c.Param("username")
	username := c.MustGet("username").(string)
	kind := c.MustGet("kind").(int)
	//fmt.Println(Pusername, username, kind) //test
	if kind == 0 || Pusername != username {
		c.JSON(http.StatusUnauthorized, gin.H{"errMsg":"you have no authority"})
		return
	}
	var coupon Coupon
	if err := c.BindJSON(&coupon); err != nil {
		log.Error(err)
		c.JSON(http.StatusBadRequest, gin.H{"errMsg":"json format wrong!"})
		return
	}
	if err := RedisClient.SAdd(username, coupon.Name).Err(); err != nil {
		log.Fatal(err)
	}
	if err := RedisClient.Set(coupon.Name, coupon.Amount, 0).Err(); err != nil {
		log.Fatal(err)
	}
	err := RedisClient.HMSet(coupon.Name + "-info", map[string]interface{}{
		"stock": coupon.Stock,
		"description":coupon.Description,
		"amount": coupon.Amount,
		}).Err();
	if err != nil {
		log.Fatal(err)
	}
	// if err := RedisClient.HSet(coupon.Name + "-info", "description", coupon.Description).Err(); err != nil {
	// 	log.Fatal(err)
	// }
	// if err := RedisClient.HSet(coupon.Name + "-info", "amount", coupon.Amount).Err(); err != nil {
	// 	log.Fatal(err)
	// }
	// all, err := RedisClient.SMembers(username).Result()
	// if err != nil {
    //     log.Fatal(err)
	// }
	// val, err := RedisClient.Get(coupon.Name).Result()
	// if err != nil {
	// 	log.Fatal(err)
	// }
	// fmt.Println("key", val)
	// fmt.Println("All member: ", all)
	insertCouponScript := "INSERT INTO coupons(username, coupons, amount, `left`, stock, description) values(?,?,?,?,?,?)"
	_ , err = db.Exec(insertCouponScript, username, coupon.Name, coupon.Amount, coupon.Amount, coupon.Stock, coupon.Description)
	if err != nil {
		log.Fatal(err)
	}
	c.JSON(http.StatusOK, gin.H{"errMsg":""})
}

func Auth() gin.HandlerFunc { 
	return func(context *gin.Context) {
		authToken := context.Request.Header.Get("Authorization")
		if authToken == "" {
			context.Abort()
			log.Info("request have no auth header")
			context.JSON(http.StatusUnauthorized, gin.H{"errMsg":"request have no auth header"})
			return
		}
		claim, err := parseToken(authToken)
		if err != nil {
			context.Abort()
			log.Fatal("something wrong with parse")
		}
		username := claim.UserName
		kind := claim.Kind
		//RedisClient.Get(user.UserName).Result() // no need, must exist ^ ^ //dont do this !!!
		context.Set("username", username)
		context.Set("kind", kind)
		context.Next()
	}
}

func makeToken(username string, kind int) string {
	token := jwt.New(jwt.SigningMethodHS256)
    claims := make(jwt.MapClaims)

    claims["Username"] = username
	claims["Kind"] = kind

    token.Claims = claims
    tokenString, _ := token.SignedString([]byte(scretKey))
    return tokenString
}

func parseToken(token string) (*myClaims, error) {
	jwtToken, err := jwt.ParseWithClaims(token, &myClaims{}, func(token *jwt.Token) (i interface{}, e error) {
		return []byte(scretKey), nil
	})
	if err == nil && jwtToken != nil {
		if claim, ok := jwtToken.Claims.(*myClaims); ok && jwtToken.Valid {
			return claim, nil
		}
	}
	return nil, err
}


func Userlogin(c *gin.Context) {
	var user User
	if err := c.BindJSON(&user); err != nil {
		log.Error(err)
		c.JSON(http.StatusUnauthorized, gin.H{"kind":"0","errMsg":"json format wrong!"})
		return
	}
	var dbpassword string
	var dbkind int
	result := db.QueryRow("select password,kind from users where username=?", user.UserName)
	if err := result.Scan(&dbpassword, &dbkind); err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusUnauthorized, gin.H{"kind":"0","errMsg":"user not exist!"})
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
		token := makeToken(user.UserName, dbkind)
		// err := RedisClient.Set(user.UserName, token, 0).Err()
		// //fmt.Println(RedisClient.Get(user.UserName).Result())
		// if err != nil {
		// 	log.Fatal("can not save token to redis")
		// }
		c.Header("Authorization", token)
		c.JSON(http.StatusOK, gin.H{"kind":dbkind,"errMsg":""})
		// fmt.Println(t)  // test 
		// tt, err := parseToken(t)
		// fmt.Println(tt, err)
		// fmt.Println(tt.UserName, tt.Kind)
		
	} else {
		c.JSON(http.StatusUnauthorized, gin.H{"kind":"0","errMsg":"password not correct!"})
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