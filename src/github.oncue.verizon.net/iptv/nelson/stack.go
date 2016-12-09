package main

import (
  "fmt"
  "errors"
  "encoding/json"
  "github.com/fatih/color"
  "github.com/parnurzeal/gorequest"
)

/////////////////// MANUAL DEPLOYMENT ///////////////////

/*
 * {
 *   "datacenter": "perryman",
 *   "namespace": "stage",
 *   "serviceType": "cassandra",
 *   "version": "1.2.3,
 *   "hash": "abcd1234",
 *   "description": "a cassandra for great good",
 *   "port": 1234
 * }
 */
type ManualDeploymentRequest struct {
  Datacenter  string `json:"datacenter"`
  Namespace   string `json:"namespace"`
  ServiceType string `json:"service_type"`
  Version     string `json:"version"`
  Hash        string `json:"hash"`
  Port        int64 `json:"port"`
  Description string `json:"description"`
}

func RegisterManualDeployment(
  req ManualDeploymentRequest,
  http *gorequest.SuperAgent, cfg *Config) (string, []error) {

  r, body, errs := AugmentRequest(
    http.Post(cfg.Endpoint+"/v1/deployments"), cfg).Send(req).EndBytes()

  if (r.StatusCode / 100 != 2){
    resp := string(body[:])
    errs = append(errs, errors.New("Unexpected response from Nelson server"))
    return resp, errs
  } else {
    return "Manual stack has been registered.", errs
  }
}

/////////////////// INSPECT ///////////////////

/*
 * {
 *   "timestamp": "2016-06-28T20:02:34.449Z",
 *   "message": "instructing perryman's chronos to handle job container",
 *   "status": "deploying"
 * }
 */
type StackStatus struct {
  Timestamp string `json:"timestamp"`
  Message string `json:"message"`
  Status string `json:"status"`
}

/*
 *   "dependencies": {
 *     "outbound": [
 *      ...
 *     ],
 *     "inbound": []
 *   },
 */
type StackDependencies struct {
  Outbound []Stack `json:"outbound"`
  Inbound []Stack `json:"inbound"`
}

/*
 * {
 *   "workflow": "pulsar",
 *   "guid": "e4184c271bb9",
 *   "statuses": [
 *     {
 *       "timestamp": "2016-07-14T22:30:22.358Z",
 *       "message": "inventory-inventory deployed to perryman",
 *       "status": "ready"
 *     },
 *     ...
 *   ],
 *   "stack_name": "inventory-inventory--2-0-11--8gufie2b",
 *   "deployed_at": 1468535384221,
 *   "unit": "inventory-inventory",
 *   "plan": "service",
 *   "expiration": 1469928212871,
 *   "dependencies": {
 *     "outbound": [
 *       {
 *         "workflow": "manual",
 *         "guid": "1a69395e919d",
 *         "stack_name": "dev-iptv-cass-dev--4-8-4--mtq2odqzndc0mg",
 *         "deployed_at": 1468518896093,
 *         "unit": "dev-iptv-cass-dev",
 *         "plan": "service"
 *       }
 *     ],
 *     "inbound": []
 *   },
 *   "namespace": "dev"
 * }
 */
type StackSummary struct {
  Workflow string `json:"workflow"`
  Guid string `json:"guid"`
  StackName string `json:"stack_name"`
  DeployedAt int64 `json:"deployed_at"`
  UnitName string `json:"unit"`
  Plan string `json:"plan"`
  NamespaceRef string `json:"namespace"`
  Expiration int64 `json:"expiration"`
  Statuses []StackStatus `json:"statuses"`
  Dependencies StackDependencies `json:"dependencies"`
}

func InspectStack(guid string, http *gorequest.SuperAgent, cfg *Config) (result StackSummary, err []error){
  r, bytes, errs := AugmentRequest(
    http.Get(cfg.Endpoint+"/v1/deployments/"+guid), cfg).EndBytes()

  if (r.StatusCode / 100 != 2){
    errs = append(errs, errors.New("Bad response from Nelson server"))
    // zomg, constructor just nulls *all* the fields, because who cares about correctness!
    return StackSummary { }, errs
  } else {
    var result StackSummary
    if err := json.Unmarshal(bytes, &result); err != nil {
      panic(err)
    }
    return result, errs
  }
}

func PrintInspectStack(s StackSummary){
  //>>>>>>>>>>> status history
  var tabulized = [][]string {}
  tabulized = append(tabulized, []string{ "GUID:", s.Guid })
  tabulized = append(tabulized, []string{ "STACK NAME:", s.StackName })
  tabulized = append(tabulized, []string{ "NAMESPACE:", s.NamespaceRef })
  tabulized = append(tabulized, []string{ "PLAN:", s.Plan })
  tabulized = append(tabulized, []string{ "WORKFLOW:", s.Workflow })
  tabulized = append(tabulized, []string{ "DEPLOYED AT:", JavaEpochToDateStr(s.DeployedAt) })
  tabulized = append(tabulized, []string{ "EXPIRES AT:", JavaEpochToDateStr(s.Expiration) })
  fmt.Println("===>> Stack Information")
  RenderTableToStdout([]string{ "Paramater", "Value" }, tabulized)

  //>>>>>>>>>>> dependency information

  green  := color.New(color.FgGreen).SprintFunc()
  yellow := color.New(color.FgYellow).SprintFunc()

  fmt.Println("") // give us a new line for spacing
  fmt.Println("===>> Dependencies")
  var dependencies = [][]string {}

  if len(s.Dependencies.Outbound) != 0 {
    for _,w := range s.Dependencies.Outbound {
      dependencies = append(dependencies,[]string{ w.Guid, w.StackName, w.Plan, w.Workflow, JavaEpochToDateStr(w.DeployedAt), yellow("OUTBOUND") })
    }
  }
  if len(s.Dependencies.Inbound) != 0 {
    for _,w := range s.Dependencies.Inbound {
      dependencies = append(dependencies,[]string{ w.Guid, w.StackName, w.Plan, w.Workflow, JavaEpochToDateStr(w.DeployedAt), green("INBOUND") })
    }
  }
  RenderTableToStdout([]string{ "GUID", "Stack", "Plan", "Workflow", "Deployed At", "Direction" }, dependencies)

  //>>>>>>>>>>> status history
  fmt.Println("") // give us a new line for spacing
  fmt.Println("===>> Status History")
  var statuslines = [][]string {}
  for _,o := range s.Statuses {
    statuslines = append(statuslines,[]string{ o.Status, o.Timestamp, o.Message })
  }
  RenderTableToStdout([]string{ "Status", "Timestamp", "Message" }, statuslines)
}

/////////////////// REDEPLOYMENT ///////////////////

func Redeploy(guid string, http *gorequest.SuperAgent, cfg *Config) (str string, err []error){
  r, body, errs := AugmentRequest(
    http.Post(cfg.Endpoint+"/v1/deployments/"+guid+"/redeploy"), cfg).EndBytes()

  if (r.StatusCode / 100 != 2){
    resp := string(body[:])
    errs = append(errs, errors.New("bad response from Nelson server"))
    return resp, errs
  } else {
    return "Redeployment requested.", errs
  }
}

/////////////////// LISTING STACKs ///////////////////

/**
 * {
 *   "workflow": "quasar",
 *   "guid": "67e04d28d6ab",
 *   "stack_name": "blobstore-testsuite--0-1-55--kbqg9nff",
 *   "deployed_at": 1467225866870,
 *   "unit": "blobstore-testsuite",
 *   "plan": "fooo",
 *   "namespace": "dev"
 * }
 */
type Stack struct {
  Workflow string `json:"workflow"`
  Guid string `json:"guid"`
  StackName string `json:"stack_name"`
  DeployedAt int64 `json:"deployed_at"`
  UnitName string `json:"unit"`
  Plan string `json:"plan"`
  NamespaceRef string `json:"namespace,omitempty"`
}

func ListStacks(delimitedDcs string, delimitedNamespaces string, delimitedStatuses string, http *gorequest.SuperAgent, cfg *Config) (list []Stack, err []error){
  uri := "/v1/deployments?"
  // set the datacenters if specified
  if (isValidCommaDelimitedList(delimitedDcs)){
    uri = uri+"dc="+delimitedDcs+"&"
  }
  if (isValidCommaDelimitedList(delimitedStatuses)){
    uri = uri+"status="+delimitedStatuses+"&"
  } else {
    // if the user didnt specify statuses, they probally only want ready units.
    uri = uri+"status=ready,warming,deprecated&"
  }
  if (isValidCommaDelimitedList(delimitedNamespaces)){
    uri = uri+"ns="+delimitedNamespaces
  } else {
    uri = uri+"ns=dev,qa,prod"
  }

  r, bytes, errs := AugmentRequest(
    http.Get(cfg.Endpoint+uri), cfg).EndBytes()

  if (r.StatusCode / 100 != 2){
    errs = append(errs, errors.New("Bad response from Nelson server"))
    return nil, errs
  } else {
    var list []Stack
    if err := json.Unmarshal(bytes, &list); err != nil {
      panic(err)
    }
    return list, errs
  }
}

func PrintListStacks(stacks []Stack){
  var tabulized = [][]string {}
  for _,s := range stacks {
    tabulized = append(tabulized,[]string{ s.Guid, s.NamespaceRef, s.StackName, s.Plan, s.Workflow, JavaEpochToDateStr(s.DeployedAt) })
  }

  RenderTableToStdout([]string{ "GUID", "Namespace", "Stack", "Plan", "Workflow", "Deployed At" }, tabulized)
}

/////////////////// DEPLOYMENT LOG ///////////////////

type StackLog struct {
  Content []string `json:"content"`
  Offset int `json:"offset"`
}

// v1/deployments/:id/log
func GetDeploymentLog(guid string, http *gorequest.SuperAgent, cfg *Config){
  _, bytes, errs := AugmentRequest(
    http.Get(cfg.Endpoint+"/v1/deployments/"+guid+"/log"), cfg).EndBytes()

  if (len(errs) > 0) {
    panic(errs)
  }

  var logs StackLog
  if err := json.Unmarshal(bytes, &logs); err != nil {
    panic(err)
  }

  fmt.Println("===>> logs for stack "+ guid)

  for _,l := range logs.Content {
    fmt.Println(l)
  }
}