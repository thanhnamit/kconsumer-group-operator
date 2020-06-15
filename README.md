# Data structure 

KConsumerGroup
  metadata
    name: String  
    namespace: String
  consumerSpec:
    - image: String: The image and version
    - topic: String
    - throughput: Int
    -
  scalingSpec:
    - minReplicas: Int: minimum number of replicas to start with
    - autoScaling: Boolean

KConsumerGroupStatus
- nodes: names of instances