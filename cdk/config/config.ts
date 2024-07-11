import configFile from "./project-config.json"

export interface ProjectConfig {
  vpcName: string
  tableName: string,
  queueName: string,
  clusterName: string,
  service: {
    name: string,
    logGroup: string,
    cpu: number,
    memory: number
    logStreamPrefix: string
  },
  dashboard:{
    name: string,
    ddbWidgetTitle: string,
    sqsWidgetTitle: string,
  }
}

const config = <ProjectConfig>configFile

export default config