package main

import (
	"bytes"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	_ "github.com/go-sql-driver/mysql"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
)

var validate = validator.New()

type AWSConfig struct {
	AccessKeyID     string
	AccessKeySecret string
	Region          string
	BucketName      string
	UploadTimeout   int
	BaseURL         string
}

type Response struct {
	Status  int                    `json:"status"`
	Message string                 `json:"message"`
	Data    map[string]interface{} `json:"data"`
}

type ImageStruct struct {
	IDServicio   int64  `json:"idServicio,omitempty" validate:"required"`
	TipoServicio int64  `json:"tipoServicio,omitempty" validate:"required"`
	Imagen       string `json:"imagen,omitempty" validate:"required"`
}

type getImage struct {
	IDServicio   int64 `json:"idServicio,omitempty"`
	TipoServicio int64 `json:"tipoServicio,omitempty"`
}

type imagen struct {
	ImagenURL string `json:"imagen,omitempty" validate:"required"`
}
type UploadImageResponse struct {
	ImagenURL string `json:"imagenURL,omitempty" validate:"required"`
}

type ArrResponse struct {
	Status  int                      `json:"status"`
	Message string                   `json:"message"`
	Data    []map[string]interface{} `json:"data"`
}

func Saludo(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"USAC": "Software Avanzado - Upload Images"})
}

func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {

		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Credentials", "true")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Header("Access-Control-Allow-Methods", "POST, HEAD, PATCH, OPTIONS, GET, PUT")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}

func MySQLConn() (db *sql.DB) {
	dbDriver := "mysql"
	dbUser := os.Getenv("DB_USER")
	dbPass := os.Getenv("DB_PASS")
	dbName := os.Getenv("DB_NAME")
	dbPort := os.Getenv("DB_PORT")
	dbHost := os.Getenv("DB_HOST")

	db, err := sql.Open(dbDriver, dbUser+":"+dbPass+"@tcp("+dbHost+":"+dbPort+")/"+dbName)
	if err != nil {
		panic(err.Error())
	}
	return db
}

func CreateSession(awsConfig AWSConfig) *session.Session {
	sess := session.Must(session.NewSession(
		&aws.Config{
			Region: aws.String(awsConfig.Region),
			Credentials: credentials.NewStaticCredentials(
				awsConfig.AccessKeyID,
				awsConfig.AccessKeySecret,
				"",
			),
		},
	))
	return sess
}

func CreateS3Session(sess *session.Session) *s3.S3 {
	s3Session := s3.New(sess)
	return s3Session
}

func CreateImage(c *gin.Context) {

	var ImagenAux ImageStruct
	db := MySQLConn()

	//validate the request body
	if err := c.BindJSON(&ImagenAux); err != nil {
		defer db.Close()
		c.JSON(http.StatusBadRequest, Response{Status: http.StatusBadRequest, Message: "error", Data: map[string]interface{}{"data": err.Error()}})
		return
	}

	//use the validator library to validate required fields
	if validationErr := validate.Struct(&ImagenAux); validationErr != nil {
		defer db.Close()
		c.JSON(http.StatusBadRequest, Response{Status: http.StatusBadRequest, Message: "error", Data: map[string]interface{}{"data": validationErr.Error()}})
		return
	}

	// INICIANDO A SUBIR IMAGEN A BUCKET S3

	id := uuid.New()
	myID := id.String()
	// myBytesArr := []byte(ImagenAux.Imagen)

	S3_ACCESS_KEY := os.Getenv("S3_ACCESS_KEY")
	S3_SECRET_KEY := os.Getenv("S3_SECRET_KEY")
	S3_AWS_REGION := os.Getenv("S3_AWS_REGION")
	S3_BUCKET_NAME := os.Getenv("S3_BUCKET_NAME")

	myAWSConfig := AWSConfig{
		AccessKeyID:     S3_ACCESS_KEY,
		AccessKeySecret: S3_SECRET_KEY,
		Region:          S3_AWS_REGION,
		BucketName:      S3_BUCKET_NAME,
		UploadTimeout:   100,
		BaseURL:         "",
	}

	mySession := CreateSession(myAWSConfig)

	imagenName := "Proyecto_Grupo5/" + myID + ".jpg"
	imagenURL := "https://" + myAWSConfig.BucketName + ".s3." + myAWSConfig.Region + ".amazonaws.com/" + imagenName

	myFinalBase64 := ImagenAux.Imagen[strings.IndexByte(ImagenAux.Imagen, ',')+1:]

	decode, errEnc := base64.StdEncoding.DecodeString(myFinalBase64)
	if errEnc != nil {
		defer db.Close()
		c.JSON(http.StatusBadRequest, Response{Status: http.StatusBadRequest, Message: "error", Data: map[string]interface{}{"data": errEnc.Error()}})
		return
	}
	// contentType := "image"
	// contentEncoding := "base64"

	uploader := s3manager.NewUploader(mySession)
	_, errS3 := uploader.Upload(&s3manager.UploadInput{
		Bucket: &myAWSConfig.BucketName,
		Key:    &imagenName,
		Body:   bytes.NewReader(decode),
	})

	if errS3 != nil {
		defer db.Close()
		c.JSON(http.StatusBadRequest, Response{Status: http.StatusBadRequest, Message: "error", Data: map[string]interface{}{"data": errS3.Error()}})
		return
	}

	// FINALIZANDO SUBIDA DE IMAGEN A BUCKET S3

	myQuery, err := db.Prepare("INSERT INTO Imagen (imagenURL, idServicio, tipoServicio) VALUES(?,?,?)")
	if err != nil {
		panic(err.Error())
	}
	res, _ := myQuery.Exec(imagenURL, ImagenAux.IDServicio, ImagenAux.TipoServicio)
	lid, _ := res.LastInsertId()
	defer db.Close()
	c.JSON(http.StatusCreated, Response{Status: http.StatusCreated, Message: "success", Data: map[string]interface{}{"resultado": "Registro de imagen creado correctamente :D", "id": lid, "imagenURL": imagenURL}})
}

func getImagen(c *gin.Context) {

	var getImage getImage
	db := MySQLConn()

	//validate the request body
	if err := c.BindJSON(&getImage); err != nil {
		defer db.Close()
		c.JSON(http.StatusBadRequest, Response{Status: http.StatusBadRequest, Message: "error", Data: map[string]interface{}{"data": err.Error()}})
		return
	}

	//use the validator library to validate required fields
	if validationErr := validate.Struct(&getImage); validationErr != nil {
		defer db.Close()
		c.JSON(http.StatusBadRequest, Response{Status: http.StatusBadRequest, Message: "error", Data: map[string]interface{}{"data": validationErr.Error()}})
		return
	}
	myQuery, err := db.Query("SELECT imagenURL FROM Imagen  where idServicio = ? and  tipoServicio = ?", getImage.IDServicio, getImage.TipoServicio)
	if err != nil {
		defer db.Close()
		c.JSON(http.StatusBadRequest, Response{Status: http.StatusBadRequest, Message: "error", Data: map[string]interface{}{"data": err.Error()}})
		return
	}

	var im imagen
	arrimagen := []imagen{}
	for myQuery.Next() {

		var imagenURL string

		err = myQuery.Scan(&imagenURL)
		if err != nil {
			defer db.Close()
			c.JSON(http.StatusBadRequest, Response{Status: http.StatusBadRequest, Message: "error", Data: map[string]interface{}{"data": err.Error()}})
			return
		}

		im.ImagenURL = imagenURL
		arrimagen = append(arrimagen, im)
	}
	var img []map[string]interface{}
	imgJson, _ := json.Marshal(arrimagen)
	json.Unmarshal(imgJson, &img)

	defer db.Close()
	c.JSON(http.StatusOK, ArrResponse{Status: http.StatusOK, Message: "success", Data: img})

}
func main() {

	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	// Se crea el servidor con GIN
	r := gin.Default()
	r.Use(CORSMiddleware())

	// Se aplican middlewares
	r.Use(gin.Logger())
	r.Use(gin.Recovery())

	//Rutas
	r.GET("/", Saludo)
	r.POST("/upload", CreateImage)
	r.POST("/getImage", getImagen)

	//Se inicia el servidor en el puerto 3006
	r.Run(":3006")
}
