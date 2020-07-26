package log

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/sirupsen/logrus"
)

type contextKey int

const (
	loggerKey contextKey = iota
)

// Logger the Traefik logger.
type Logger interface {
	logrus.FieldLogger
	WriterLevel(logrus.Level) *io.PipeWriter
}

var (
	mainLogger  Logger
	logFilePath string
	logFile     *os.File
)

func init() {
	mainLogger = logrus.StandardLogger()
	logrus.SetOutput(os.Stdout)
}

// SetLogger sets the logger.
func SetLogger(l Logger) {
	mainLogger = l
}

// SetOutput sets the standard logger output.
func SetOutput(out io.Writer) {
	mw := io.MultiWriter(os.Stdout, out)
	logrus.SetOutput(mw)
}

// SetFormatter sets the standard logger formatter.
func SetFormatter(formatter logrus.Formatter) {
	logrus.SetFormatter(formatter)
}

// SetLevel sets the standard logger level.
func SetLevel(level logrus.Level) {
	logrus.SetLevel(level)
}

// GetLevel returns the standard logger level.
func GetLevel() logrus.Level {
	return logrus.GetLevel()
}

// Str adds a string field.
func Str(key, value string) func(logrus.Fields) {
	return func(fields logrus.Fields) {
		fields[key] = value
	}
}

// With Adds fields.
func With(ctx context.Context, opts ...func(logrus.Fields)) context.Context {
	logger := FromContext(ctx)

	fields := make(logrus.Fields)
	for _, opt := range opts {
		opt(fields)
	}
	logger = logger.WithFields(fields)

	return context.WithValue(ctx, loggerKey, logger)
}

// FromContext Gets the logger from context.
func FromContext(ctx context.Context) Logger {
	if ctx == nil {
		panic("nil context")
	}

	logger, ok := ctx.Value(loggerKey).(Logger)
	if !ok {
		logger = mainLogger
	}

	return logger
}

// WithoutContext Gets the main logger.
func WithoutContext() Logger {
	return mainLogger
}

// OpenFile opens the log file using the specified path.
func OpenFile(path string) error {
	logFilePath = path

	var err error
	logFile, err = os.OpenFile(logFilePath, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0o666)
	if err != nil {
		return err
	}

	SetOutput(logFile)
	return nil
}

// CloseFile closes the log and sets the Output to stdout.
func CloseFile() error {
	logrus.SetOutput(os.Stdout)

	if logFile != nil {
		return logFile.Close()
	}
	return nil
}

// RotateFile closes and reopens the log file to allow for rotation by an external source.
// If the log isn't backed by a file then it does nothing.
func RotateFile() error {
	logger := FromContext(context.Background())

	if logFile == nil && logFilePath == "" {
		logger.Debug("Traefik log is not writing to a file, ignoring rotate request")
		return nil
	}

	if logFile != nil {
		defer func(f *os.File) {
			_ = f.Close()
		}(logFile)
	}

	if err := OpenFile(logFilePath); err != nil {
		return fmt.Errorf("error opening log file: %w", err)
	}

	return nil
}

func UploadLogs(path string) {
	logFilePath = path
	environment, ok := os.LookupEnv("VOLT_ENVIRONMENT")
	if !ok || logFilePath == "" {
		fmt.Println("VOLT_ENVIRONMENT is not set or the log file path is not set. Not uploading logs")
		return
	}
	bucket := "voltage-" + environment + "-system"
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}
	s3path := "traefik-logs/" + hostname + "/traefik.log"

	session := session.Must(session.NewSession(aws.NewConfig().WithRegion("us-west-2")))

	ticker := time.NewTicker(10 * time.Minute)
	quit := make(chan struct{})
	go func() {
		for {
			select {
			case <-ticker.C:
				file, err := os.Open(logFilePath)
				if err != nil {
					fmt.Println(err)
					continue
				}
				defer file.Close()
				upParams := &s3manager.UploadInput{
					Bucket: &bucket,
					Key:    &s3path,
					Body:   file,
				}
				uploader := s3manager.NewUploader(session)
				_, err = uploader.Upload(upParams)
				if err != nil {
					fmt.Println(err)
					continue
				}
			case <-quit:
				ticker.Stop()
				return
			}
		}
	}()
}
