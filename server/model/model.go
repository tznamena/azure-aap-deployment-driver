package model

import (
	"encoding/json"

	"time"

	log "github.com/sirupsen/logrus"
	"gorm.io/datatypes"
	_ "gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// Replicate GORM base model, hiding times from json
type BaseModel struct {
	ID        uint      `gorm:"primaryKey"`
	CreatedAt time.Time `json:"-"`
	UpdatedAt time.Time `json:"-"`
	DeletedAt time.Time `json:"-" gorm:"index"`
}

type Step struct {
	BaseModel
	Name       string            `gorm:"unique" json:"name"`
	Template   datatypes.JSONMap `json:"-"`
	Parameters datatypes.JSONMap `json:"-"`
	Priority   uint              `json:"order"`
	Executions []Execution       `json:"executions" gorm:"constraint:OnUpdate:CASCADE;"`
}

type Output struct {
	BaseModel
	ModuleName string `json:"moduleName"`
	Values     datatypes.JSONMap
}

type Execution struct {
	BaseModel
	Status            ExecutionStatus `json:"status" gorm:"type:string"`
	StepID            uint            `json:"stepId"`
	DeploymentID      string          `json:"-"`
	Error             string          `json:"error"`
	ErrorDetails      string          `json:"errorDetails"`
	Code              string          `json:"code"`
	ProvisioningState string          `json:"provisioningState"`
	Details           string          `json:"details"`
	Timestamp         time.Time       `json:"timestamp"`
	Duration          string          `json:"duration"`
	CorrelationID     string          `json:"correlationId"`
	ResumeToken       string          `json:"-"`
}

type Status struct {
	BaseModel
	TemplatesLoaded   bool
	MainOutputsLoaded bool
	IsFatalState      bool
	FirstStart        time.Time
}

type EngineConfiguration struct {
	StepRestartTimeout    int64 `json:"stepRestartTimeoutSec"`
	OverallTimeout        int64 `json:"overallTimeoutSec"`
	EngineExitDelay       int64 `json:"engineExitDelaySec"`
	AutoRetryDelay        int64 `json:"autoRetryDelaySec"`
	StepDeploymentTimeout int64 `json:"stepDeploymentTimeoutSec"`
	StepMaxRetries        int   `json:"stepMaxRetries"`
}

type SessionConfig struct {
	BaseModel
	SessionAuthKey []byte
}

// DB comparisons don't work well with empty string, so
// use this to mark "top level" telemetry values
const MAIN_MARKER string = "xxmainxx"

type Telemetry struct {
	MetricName  DeploymentMetric `gorm:"type:string;primaryKey"`
	MetricValue string
	Step        string `gorm:"primaryKey"`
}

type RedHatEntitlements struct {
	BaseModel
	Sku                string
	SubscriptionNumber string
}

type AzureMarketplaceEntitlement struct {
	BaseModel
	AzureSubscriptionId string
	AzureCustomerId     string
	RHEntitlements      []RedHatEntitlements `gorm:"foreignKey:ID"`
	RedHatAccountId     string
	Status              string
	ErrorMessage        string
}

func UpdateExecution(execution *Execution, result *DeploymentResult, errJson string) {
	execution.ResumeToken = ""

	if result != nil {
		// Failed during deployment
		execution.Status = result.Status
		execution.DeploymentID = result.ID
		execution.CorrelationID = result.CorrelationID
		if result.Duration != "" {
			execution.Duration = GetAzureTimeFormatted(result.Duration)
		}
		execution.Timestamp = result.Timestamp
		execution.ProvisioningState = result.ProvisioningState
	} else {
		// Failed before deployment was created
		execution.Status = Failed
	}

	if errJson != "" {
		errorStruct := ErroredDeployment{}
		err := json.Unmarshal([]byte(errJson), &errorStruct)
		if err != nil {
			log.Warnf("Unable to parse Azure error: %v", err)
			execution.Error = err.Error()
			return
		}
		execution.Error = errorStruct.Error.Message
		execution.ErrorDetails = errorStruct.Error.DetailString()
		execution.Code = errorStruct.Error.Code
	}
}

func CreateNewOutput(name string, result *DeploymentResult) *Output {
	return &Output{
		ModuleName: name,
		Values:     result.Outputs,
	}
}

// Setter function for each deployment metric
func SetMetric(db *gorm.DB, metric DeploymentMetric, value string, step string) {
	db.Save(&Telemetry{
		MetricName:  metric,
		MetricValue: value,
		Step:        step,
	})
}

// Getter function for each deployment metric
func Metric(db *gorm.DB, metric DeploymentMetric) Telemetry {
	telemetry := Telemetry{}
	db.Where("metric_name = ?", metric).Find(&telemetry)
	return telemetry
}
