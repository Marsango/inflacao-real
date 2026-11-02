package scraperhealth

import (
	"database/sql"
	"encoding/json"
	"strconv"

	"github.com/gin-gonic/gin"
)

func RegisterAdmin(routes *gin.RouterGroup, database *sql.DB, scraperID string, shops []string) {
	routes.GET("/scrapers", func(request *gin.Context) { listRuns(request, database, scraperID, shops) })
	routes.GET("/scraper-diagnostics", func(request *gin.Context) { listDiagnostics(request, database) })
	routes.DELETE("/scraper-diagnostics", func(request *gin.Context) { deleteDiagnostics(request, database) })
}
func listRuns(request *gin.Context, database *sql.DB, scraperID string, shops []string) {
	rows, err := database.QueryContext(request.Request.Context(), runsSQL, scraperID, shops)
	if err != nil {
		request.AbortWithStatus(503)
		return
	}
	defer rows.Close()
	writeDiagnostics(request, rows)
}

const runsSQL = `WITH identities AS (
 SELECT $1::text scraper_id, id shop_id FROM shops WHERE id=ANY($2::text[])
 UNION SELECT scraper_id,shop_id FROM scraper_runs)
 SELECT jsonb_build_object('scraper_id',i.scraper_id,'shop_id',i.shop_id,
 'scraper_name',coalesce(r.scraper_name,s.name),'started_at',r.started_at,'finished_at',r.finished_at,
 'observations',r.observations,'errors',r.errors,'warnings',r.warnings,'omitted',r.omitted,
 'status',CASE WHEN r.run_id IS NULL THEN 'unknown' WHEN r.errors>0 THEN 'error' WHEN r.warnings>0 THEN 'warning' ELSE 'healthy' END)
 FROM identities i JOIN shops s ON s.id=i.shop_id LEFT JOIN LATERAL (
 SELECT * FROM scraper_runs WHERE scraper_id=i.scraper_id AND shop_id=i.shop_id ORDER BY started_at DESC,run_id DESC LIMIT 1
 ) r ON true ORDER BY s.name,i.scraper_id`

func listDiagnostics(request *gin.Context, database *sql.DB) {
	severity := request.Query("severity")
	if severity != "" && severity != "error" && severity != "warning" {
		request.AbortWithStatus(400)
		return
	}
	before, err := strconv.ParseInt(request.DefaultQuery("before", "9223372036854775807"), 10, 64)
	if err != nil || before < 1 {
		request.AbortWithStatus(400)
		return
	}
	rows, err := database.QueryContext(request.Request.Context(), diagnosticsSQL, request.Query("scraper_id"), request.Query("shop_id"), severity, before)
	if err != nil {
		request.AbortWithStatus(503)
		return
	}
	defer rows.Close()
	writeDiagnostics(request, rows)
}

const diagnosticsSQL = `SELECT jsonb_build_object('id',d.id::text,'run_id',d.run_id,'severity',d.severity,'category',d.category,'message',d.message,'started_at',r.started_at)
 FROM scraper_diagnostics d JOIN scraper_runs r USING(scraper_id,shop_id,run_id)
 WHERE d.scraper_id=$1 AND d.shop_id=$2 AND ($3='' OR d.severity=$3) AND d.id<$4 ORDER BY d.id DESC LIMIT 50`

func writeDiagnostics(request *gin.Context, rows *sql.Rows) {
	result, err := readDiagnostics(rows)
	if err != nil {
		request.AbortWithStatus(503)
		return
	}
	request.JSON(200, result)
}
func readDiagnostics(rows *sql.Rows) ([]json.RawMessage, error) {
	result := []json.RawMessage{}
	var err error
	for rows.Next() && err == nil {
		var value json.RawMessage
		err = rows.Scan(&value)
		result = append(result, value)
	}
	if err != nil {
		return nil, err
	}
	return result, rows.Err()
}

func deleteDiagnostics(request *gin.Context, database *sql.DB) {
	identity := request.Query("id")
	if invalidIdentity(identity) {
		request.AbortWithStatus(400)
		return
	}
	if request.Query("scraper_id") == "" || request.Query("shop_id") == "" {
		request.AbortWithStatus(400)
		return
	}
	_, err := database.ExecContext(request.Request.Context(), `DELETE FROM scraper_diagnostics WHERE scraper_id=$1 AND shop_id=$2 AND ($3='' OR id::text=$3)`, request.Query("scraper_id"), request.Query("shop_id"), identity)
	if err != nil {
		request.AbortWithStatus(503)
		return
	}
	request.Status(204)
}

func invalidIdentity(identity string) bool {
	if identity == "" {
		return false
	}
	value, err := strconv.ParseInt(identity, 10, 64)
	return err != nil || value < 1
}
