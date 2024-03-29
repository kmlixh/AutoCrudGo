package AutoCrudGo

import (
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/kmlixh/gom/v3"
	"github.com/kmlixh/gom/v3/define"
	"net/http"
	"reflect"
	"strconv"
	"strings"
)

type ConditionParam struct {
	QueryName string
	ColName   string
	Required  bool
}

type RouteHandler struct {
	Path       string
	HttpMethod string
	Handlers   []gin.HandlerFunc
}

func RegisterHandler(name string, routes gin.IRoutes, handlers ...RouteHandler) error {
	if handlers == nil || len(handlers) == 0 {
		return errors.New("route handler could not be empty or nil")
	}
	for _, handler := range handlers {
		if handler.HttpMethod != "Any" {
			routes.Handle(handler.HttpMethod, name+"/"+handler.Path, handler.Handlers...)
		} else {
			routes.Any(name+"/"+handler.Path, handler.Handlers...)
		}
	}
	return nil
}

func RegisterDefaultLite(prefix string, i any, routes gin.IRoutes, db *gom.DB) error {
	columnNames, primaryKeys, primaryAuto, columnIdxMap := gom.GetColumns(reflect.ValueOf(i))
	if len(columnNames) > 0 {
		return RegisterDefault(prefix, i, routes, db, append(primaryKeys, append(primaryAuto, columnNames...)...), append(primaryKeys, append(primaryAuto, columnNames...)...), append(primaryKeys, columnNames...), columnNames, columnIdxMap)
	} else {
		return errors.New("Struct was empty")
	}

}
func RegisterDefault(prefix string, i any, routes gin.IRoutes, db *gom.DB, queryCols []string, queryDetailCols []string, insertCols []string, updateCols []string, columnFieldMap map[string]string) error {
	listHandler := GetQueryListHandler(i, db, GetConditionParam(queryCols, columnFieldMap, i), queryCols)
	insertHandler := GetInsertHandler(i, db, insertCols)
	detailHandler := GetQuerySingleHandler(i, db, GetConditionParam(queryDetailCols, columnFieldMap, i), queryDetailCols)
	updateHandler := GetUpdateHandler(i, db, GetConditionParam(updateCols, columnFieldMap, i), updateCols)
	deleteHandler := GetDeleteHandler(i, db, GetConditionParam(updateCols, columnFieldMap, i))
	return RegisterHandler(prefix, routes, listHandler, insertHandler, detailHandler, updateHandler, deleteHandler)
}
func GetConditionParam(columns []string, columnFieldMap map[string]string, i any) []ConditionParam {
	t := reflect.TypeOf(i)
	params := make([]ConditionParam, 0)
	for _, col := range columns {
		fieldName := columnFieldMap[col]
		f, ok := t.FieldByName(fieldName)
		if ok {
			panic(errors.New(fmt.Sprintf(" [%s] was not exist! ", fieldName)))
		}
		params = append(params, GenDefaultConditionParamByType(col, f))
	}
	return params
}
func GenDefaultConditionParamByType(column string, f reflect.StructField) ConditionParam {
	return ConditionParam{
		QueryName: GetFieldDefaultQueryName(f),
		ColName:   column,
		Required:  false,
	}
}
func GetFieldDefaultQueryName(f reflect.StructField) string {
	return strings.ToLower(f.Name[0:1]) + f.Name[1:]
}
func GetQueryListHandler(i any, db *gom.DB, queryParam []ConditionParam, columns []string) RouteHandler {
	return RouteHandler{
		Path:       "list",
		HttpMethod: "Any",
		Handlers:   []gin.HandlerFunc{QueryList(i, db, queryParam, columns)},
	}
}
func GetQuerySingleHandler(i any, db *gom.DB, queryParam []ConditionParam, columns []string) RouteHandler {
	return RouteHandler{
		Path:       "detail",
		HttpMethod: http.MethodGet,
		Handlers:   []gin.HandlerFunc{QuerySingle(i, db, queryParam, columns)},
	}
}
func GetInsertHandler(i any, db *gom.DB, columns []string) RouteHandler {
	return RouteHandler{
		Path:       "add",
		HttpMethod: http.MethodPost,
		Handlers:   []gin.HandlerFunc{DoInsert(i, db, columns)},
	}
}
func GetUpdateHandler(i any, db *gom.DB, queryParam []ConditionParam, columns []string) RouteHandler {
	return RouteHandler{
		Path:       "update",
		HttpMethod: http.MethodPost,
		Handlers:   []gin.HandlerFunc{DoUpdate(i, db, queryParam, columns)},
	}
}
func GetDeleteHandler(i any, db *gom.DB, queryParam []ConditionParam) RouteHandler {
	return RouteHandler{
		Path:       "delete",
		HttpMethod: http.MethodPost,
		Handlers:   []gin.HandlerFunc{DoDelete(i, db, queryParam)},
	}
}

func QueryList(i interface{}, db *gom.DB, queryParam []ConditionParam, columns []string) gin.HandlerFunc {
	return func(c *gin.Context) {
		codeMsg := Ok()
		results := reflect.New(reflect.SliceOf(reflect.TypeOf(i))).Interface()
		cnd, maps, err := MapToParamCondition(c, queryParam)

		pageNum := int64(0)
		pageSize := int64(20)
		pNum, hasPageNum := maps["pageNum"]
		pSize, hasPageSize := maps["pageSize"]
		if hasPageSize {
			switch pNum.(type) {
			case string:
				t, er := strconv.Atoi(pSize.(string))
				if er == nil {
					pageSize = int64(t)
				}
			default:
				pageSize = pSize.(int64)
			}
		}
		if hasPageNum {
			switch pNum.(type) {
			case string:
				t, er := strconv.Atoi(pNum.(string))
				if er == nil {
					pageNum = int64(t)
				}
			default:
				pageNum = pNum.(int64)
			}
			if cnd != nil && err == nil {
				db.Where(cnd)
			}
			count, err := db.Count("*")
			if err != nil {
				RenderErr2(c, 500, "统计数据出错！")
				return
			}
			codeMsg.Set("total", count)
			codeMsg.Set("pageNum", pageNum)
			codeMsg.Set("pageSize", pageSize)
			db.Page(pageNum, pageSize)
		}
		if cnd != nil && err == nil {
			db.Where(cnd)
		}
		_, err = db.Select(results, columns...)
		if err != nil {
			RenderErr2(c, 500, "服务器内部错误："+err.Error())
			return
		}
		codeMsg.SetData(results)
		RenderJson(c, codeMsg)
	}
}
func QuerySingle(i any, db *gom.DB, queryParam []ConditionParam, columns []string) gin.HandlerFunc {
	return func(c *gin.Context) {
		results := reflect.New(reflect.TypeOf(i)).Interface()
		cnd, _, err := MapToParamCondition(c, queryParam)
		if cnd != nil && err == nil {
			db.Where(cnd)
		}
		_, err = db.Select(results, columns...)
		if err != nil {
			RenderErr2(c, 500, "服务器内部错误："+err.Error())
			return
		}
		RenderOk(c, results)
	}
}
func DoUpdate(i any, db *gom.DB, param []ConditionParam, columns []string) gin.HandlerFunc {
	return func(c *gin.Context) {
		cnd, _, er := MapToParamCondition(c, param)
		if er != nil {
			RenderErrs(c, er)
			return
		}
		if cnd != nil {
			db.Where(cnd)
		}
		results := reflect.New(reflect.TypeOf(i)).Interface()
		err := c.ShouldBind(&results)
		if err != nil {
			RenderErrs(c, err)
			return
		}
		rs, er := db.Insert(results, columns...)
		if er != nil {
			RenderErrs(c, err)
			return
		}
		RenderOk(c, rs)
	}
}
func DoInsert(i any, db *gom.DB, columns []string) gin.HandlerFunc {
	return func(c *gin.Context) {
		results := reflect.New(reflect.TypeOf(i)).Interface()
		err := c.ShouldBind(&results)
		if err != nil {
			RenderErrs(c, err)
			return
		}
		rs, er := db.Insert(results, columns...)
		if er != nil {
			RenderErrs(c, err)
			return
		}
		RenderOk(c, rs)
	}
}
func DoDelete(i any, db *gom.DB, param []ConditionParam) gin.HandlerFunc {
	return func(c *gin.Context) {
		cnd, _, er := MapToParamCondition(c, param)
		if er != nil {
			RenderErrs(c, er)
			return
		}
		rs, er := db.Where(cnd).Delete(i)
		if er != nil {
			RenderErrs(c, er)
			return
		}
		RenderOk(c, rs)
	}
}

var operators = []string{"Eq", "Le", "Lt", "Ge", "Gt", "Like", "LikeLeft", "LikeRight", "In", "NotIn", "NotLike", "NotEq"}

func MapToParamCondition(c *gin.Context, queryParam []ConditionParam) (define.Condition, map[string]interface{}, error) {
	maps, err := GetConditionMapFromRst(c)
	hasValMap := make(map[string]string)
	if err != nil {
		return nil, nil, err
	}
	if len(maps) > 0 && len(queryParam) > 0 {
		var cnd = gom.CndEmpty()
		for _, param := range queryParam {
			oldName, hasOldVal := hasValMap[param.QueryName]
			if hasOldVal {
				return nil, nil, errors.New(fmt.Sprintf("u have a query condition like [%s]", oldName))
			}
			for _, oper := range operators {
				val, hasVal := maps[param.QueryName+oper]
				if hasVal {
					hasValMap[param.ColName] = param.QueryName + oper
					switch oper {
					case "Eq":
						cnd.Eq(param.ColName, val)
					case "NotEq":
						cnd.NotEq(param.ColName, val)
					case "Le":
						cnd.Le(param.ColName, val)
					case "Lt":
						cnd.Lt(param.ColName, val)
					case "Ge":
						cnd.Ge(param.ColName, val)
					case "Gt":
						cnd.Gt(param.ColName, val)
					case "Like":
						cnd.Like(param.ColName, val)
					case "LikeLeft":
						cnd.LikeIgnoreStart(param.ColName, val)
					case "LikeRight":
						cnd.LikeIgnoreEnd(param.ColName, val)
					case "In":
						cnd.In(param.ColName, gom.UnZipSlice(val)...)
					case "NotIn":
						cnd.NotIn(param.ColName, gom.UnZipSlice(val)...)
					case "NotLike":
						cnd.NotLike(param.ColName, val)
					}
				}
			}

		}
		return cnd, maps, nil
	} else {
		return nil, nil, nil
	}
}
