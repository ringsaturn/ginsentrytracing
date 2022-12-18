package main

import (
	"fmt"
	"net/http"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/gin-gonic/gin"
	"github.com/ringsaturn/ginsentrytracing"
)

func init() {
	err := sentry.Init(sentry.ClientOptions{
		EnableTracing:    true,
		TracesSampleRate: 1,
	})
	if err != nil {
		panic(err)
	}
}

func Foo(ctx *gin.Context) {
	span := ginsentrytracing.StartSpanFromGinContext(ctx, "slow opration")
	for i := 0; i < 5; i++ {
		childOp := span.StartChild(fmt.Sprintf("slow op: %v", i))
		time.Sleep(100 * time.Millisecond)
		childOp.Finish()
	}
	span.Finish()
	ctx.String(http.StatusOK, "bar")
	sentry.Flush(time.Second)
}

func main() {
	app := gin.Default()
	app.Use(ginsentrytracing.AttachSpan(ginsentrytracing.GetTraceAndBaggageFromRequest))

	app.GET("/foo", Foo)
	app.Run()
}
