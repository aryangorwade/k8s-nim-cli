nvidia@nim-operator-72qbr53:~$ kl describe nimcache meta-llama3-8b-instruct
Name:         meta-llama3-8b-instruct
Namespace:    default
Labels:       <none>
Annotations:  <none>
API Version:  apps.nvidia.com/v1alpha1
Kind:         NIMCache
Metadata:
  Creation Timestamp:  2025-08-21T00:11:33Z
  Finalizers:
    finalizer.nimcache.apps.nvidia.com
  Generation:        2
  Resource Version:  54433544
  UID:               978da86e-1ee3-4ec1-9cd8-309824288068
Spec:
  Resources:
    Cpu:     0
    Memory:  0
  Source:
    Ngc:
      Auth Secret:  ngc-api-secret
      Model:
        Engine:              tensorrt_llm
        Tensor Parallelism:  1
      Model Puller:          nvcr.io/nim/meta/llama-3.1-8b-instruct:1.3.3
      Pull Secret:           ngc-secret
  Storage:
    Pvc:
      Create:              true
      Size:                50Gi
      Storage Class:       nim-sc
      Volume Access Mode:  ReadWriteMany
Status:
  Conditions:
    Last Transition Time:  2025-08-21T00:11:35Z
    Message:               The Job to cache NIM is in progress
    Reason:                JobRunning
    Status:                False
    Type:                  NIM_CACHE_JOB_PENDING
  State:                   InProgress
Events:                    <none>