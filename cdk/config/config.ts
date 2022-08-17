import configFile from "./project-config.json"

export interface ProjectConfig {

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
}

const config = <ProjectConfig>configFile

export default config