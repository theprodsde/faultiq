import { config } from "@/config"

export interface HealthCheckResponse {
  status: "healthy" | "degraded" | "unhealthy"
  timestamp: string
  services: Record<
    string,
    {
      status: "healthy" | "degraded" | "unhealthy"
      responseTime: number
      uptime: number
    }
  >
}

class HealthCheckService {
  private cache: Map<string, { data: any; timestamp: number }> = new Map()
  private readonly CACHE_DURATION = 30000 // 30 seconds

  async checkServiceHealth(
    serviceUrl: string
  ): Promise<{
    status: "healthy" | "degraded" | "unhealthy"
    responseTime: number
  }> {
    const now = Date.now()
    const startTime = now

    try {
      const response = (await Promise.race([
        fetch(`${serviceUrl}/health`, { method: "GET" }),
        new Promise((_, reject) =>
          setTimeout(() => reject(new Error("Timeout")), 5000)
        ),
      ])) as Response

      const responseTime = Date.now() - startTime

      if (!response || ("ok" in response && !response.ok)) {
        return {
          status: "unhealthy",
          responseTime,
        }
      }

      // Determine status based on response time
      if (responseTime > 3000) {
        return { status: "degraded", responseTime }
      }

      return { status: "healthy", responseTime }
    } catch (error) {
      return {
        status: "unhealthy",
        responseTime: Date.now() - startTime,
      }
    }
  }

  async checkAllServices(): Promise<HealthCheckResponse> {
    const services = {
      "api-gateway": `${config.apiUrl}`,
      "detection-engine": `${config.detectionEngineUrl}`,
      "graph-manager": `${config.graphManagerUrl}`,
      neo4j: `${config.neo4jUrl}`,
      postgres: `${config.postgresUrl}`,
      redis: `${config.redisUrl}`,
    }

    const results = await Promise.all(
      Object.entries(services).map(async ([name, url]) => {
        const { status, responseTime } = await this.checkServiceHealth(url)
        return {
          name,
          status,
          responseTime,
          uptime: 99.9, // TODO: Fetch from backend
        }
      })
    )

    const servicesStatus: Record<
      string,
      {
        status: "healthy" | "degraded" | "unhealthy"
        responseTime: number
        uptime: number
      }
    > = {}
    results.forEach((result) => {
      servicesStatus[result.name] = {
        status: result.status,
        responseTime: result.responseTime,
        uptime: result.uptime,
      }
    })

    const allHealthy = results.every((r) => r.status === "healthy")
    const someDegraded = results.some((r) => r.status === "degraded")

    return {
      status: allHealthy ? "healthy" : someDegraded ? "degraded" : "unhealthy",
      timestamp: new Date().toISOString(),
      services: servicesStatus,
    }
  }

  async getServiceHealthCached(): Promise<HealthCheckResponse> {
    const cached = this.cache.get("health-check")
    const now = Date.now()

    if (cached && now - cached.timestamp < this.CACHE_DURATION) {
      return cached.data
    }

    const data = await this.checkAllServices()
    this.cache.set("health-check", { data, timestamp: now })
    return data
  }
}

export const healthCheckService = new HealthCheckService()
