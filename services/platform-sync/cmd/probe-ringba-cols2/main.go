package main
import (
  "bytes"; "context"; "encoding/json"; "fmt"; "io"; "net/http"; "os"; "strings"; "time"
  "github.com/Syed-Tajdar/calyx-ai/services/platform-sync/internal/crypto"
  "github.com/jackc/pgx/v5/pgxpool"
)
func main() {
  key:=os.Getenv("CREDENTIAL_ENCRYPTION_KEY"); dsn:=os.Getenv("DATABASE_URL")
  ctx,_:=context.WithTimeout(context.Background(),90*time.Second)
  pool,_:=pgxpool.New(ctx,dsn); defer pool.Close()
  _,_=pool.Exec(ctx,`SELECT set_config('app.current_tenant',$1,false)`,"11111111-1111-1111-1111-111111111111")
  var blob []byte
  _=pool.QueryRow(ctx,`SELECT api_credentials_blob FROM integrations WHERE id='ae17d28e-3f98-4288-abf9-ef1bfa978776'::uuid`).Scan(&blob)
  dec,_:=crypto.NewCredentialDecryptor(key); creds,_:=dec.DecryptMap(blob)
  token:=creds["api_token"]; if token=="" { token=creds["api_key"] }
  url:="https://api.ringba.com/v2/"+creds["account_id"]+"/calllogs"
  end:=time.Now().UTC(); start:=end.Add(-7*24*time.Hour)
  client:=&http.Client{Timeout:30*time.Second}
  cands:=[]string{
    "hasConverted","wasConverted","conversion","conversionCount","convertedCount",
    "isConvertedCall","callConverted","Converted","hasConversion",
    "callConnectedTime","callCompleteTime","connectedTime","completeTime",
    "callEndDt","endDt","answerDt","answeredDt","callAnswerDt",
    "dtConnected","dtComplete","callEnd","endTime","startTime",
  }
  for _,col:=range cands {
    vc:=[]map[string]string{{"column":"inboundCallId"},{"column":col}}
    body,_:=json.Marshal(map[string]interface{}{
      "reportStart":start.Format(time.RFC3339),"reportEnd":end.Format(time.RFC3339),
      "filters":[]interface{}{},"size":3,"offset":0,"valueColumns":vc,
    })
    req,_:=http.NewRequest("POST",url,bytes.NewReader(body))
    req.Header.Set("Authorization","Token "+token); req.Header.Set("Content-Type","application/json")
    resp,err:=client.Do(req); if err!=nil { fmt.Println("err",err); continue }
    raw,_:=io.ReadAll(io.LimitReader(resp.Body,1<<20)); resp.Body.Close()
    if resp.StatusCode!=200 {
      var m struct{ Message string `json:"message"` }
      _=json.Unmarshal(raw,&m)
      fmt.Printf("REJECT %s %s\n", col, m.Message)
    } else {
      var p struct{ Report struct{ Records []map[string]interface{} `json:"records"` } `json:"report"` }
      _=json.Unmarshal(raw,&p)
      present:=false; sample:=""
      if len(p.Report.Records)>0 {
        for k,v:=range p.Report.Records[0] {
          if strings.EqualFold(k,col) { present=true; sample=fmt.Sprintf("%v",v) }
        }
      }
      if present { fmt.Printf("ACCEPT %s sample=%s\n", col, sample) } else { fmt.Printf("SILENT %s\n", col) }
    }
    time.Sleep(220*time.Millisecond)
  }
}
