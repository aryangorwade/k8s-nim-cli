package util

// NIMService values.
const (
	// Null values for these because they are needed.
	ImageRepository = 	""
	Tag 			= 	""
	NIMCacheStorage = 	""
	PVCStorage		= 	""
	
	// Default values.
	PullPolicy		=  "IfNotPresent"
	PullSecret		=  "ngc-secret"
	AuthSecret		=  "ngc-api-secret"
	ServicePort		=  8000
	ServiceType		= "ClusterIP"
	GPULimit		=  "1"
)

// NIMCache values. 
const (

)