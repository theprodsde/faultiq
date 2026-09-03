"use client"

import { PublicLayout } from "@/components/layouts/PublicLayout"
import { Button } from "@/components/ui/button"
import Link from "next/link"
import { motion } from "framer-motion"
import { ArrowRight, Zap, BarChart3, Shield, Clock, Users, Code } from "lucide-react"
import dynamic from "next/dynamic"

const SystemArchitectureDiagram = dynamic(
  () => import("@/components/diagrams/SystemArchitectureDiagram"),
  { ssr: false }
)

const features = [
  {
    icon: <Zap className="w-6 h-6" />,
    title: "Real-time Detection",
    description: "Instantly detect API failures and anomalies with advanced AI algorithms",
  },
  {
    icon: <BarChart3 className="w-6 h-6" />,
    title: "Visual Analytics",
    description: "Comprehensive dashboards with metrics, trends, and correlations",
  },
  {
    icon: <Shield className="w-6 h-6" />,
    title: "Enterprise Security",
    description: "SOC 2 Type II compliant with end-to-end encryption",
  },
  {
    icon: <Clock className="w-6 h-6" />,
    title: "Fast Response",
    description: "Reduce MTTR with automated incident detection and alerting",
  },
  {
    icon: <Users className="w-6 h-6" />,
    title: "Team Collaboration",
    description: "Multi-tenant platform with role-based access control",
  },
  {
    icon: <Code className="w-6 h-6" />,
    title: "API-First Design",
    description: "Integrate with your existing systems seamlessly",
  },
]

export default function Home() {
  return (
    <PublicLayout>
      {/* Hero Section */}
      <section className="relative overflow-hidden py-20 md:py-32 bg-gradient-to-br from-slate-900 via-blue-900 to-slate-900 dark:from-slate-950 dark:via-blue-950 dark:to-slate-950">
        <div className="absolute inset-0 bg-grid-white/5" />
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 relative z-10">
          <motion.div
            initial={{ opacity: 0, y: 20 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ duration: 0.8 }}
            className="text-center"
          >
            <div className="inline-block mb-4">
              <span className="px-4 py-2 rounded-full bg-blue-500/20 border border-blue-400/30 text-sm font-medium text-blue-200">
                🚀 Enterprise API Intelligence
              </span>
            </div>
            <h1 className="text-5xl md:text-6xl font-bold text-white mb-6 leading-tight">
              Stop Guessing.<br />
              <span className="bg-gradient-to-r from-blue-400 to-cyan-400 bg-clip-text text-transparent">
                Start Knowing.
              </span>
            </h1>
            <p className="text-xl text-slate-300 mb-8 max-w-2xl mx-auto">
              Detect API failures before they impact your users. FaultIQ provides real-time incident detection, root cause analysis, and intelligent recommendations.
            </p>
            <div className="flex flex-col sm:flex-row gap-4 justify-center">
              <Link href="/login">
                <Button size="lg" className="gap-2">
                  Get Started <ArrowRight className="w-4 h-4" />
                </Button>
              </Link>
              <Link href="/about">
                <Button size="lg" variant="outline" className="text-white border-white/20 hover:bg-white/10">
                  Learn More
                </Button>
              </Link>
            </div>
          </motion.div>

          {/* System Architecture Diagram */}
          <motion.div
            initial={{ opacity: 0, y: 30 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ duration: 1, delay: 0.5 }}
            className="mt-16 relative h-[550px] md:h-[650px] rounded-2xl overflow-hidden"
          >
            <SystemArchitectureDiagram />
            <p className="absolute bottom-3 left-1/2 -translate-x-1/2 text-xs text-slate-400 font-mono bg-black/50 px-3 py-1 rounded-full backdrop-blur-sm">
              Live system simulation — failures auto-detected &amp; resolved by FaultIQ
            </p>
          </motion.div>
        </div>
      </section>

      {/* Features Section */}
      <section className="py-20 md:py-32 bg-white dark:bg-slate-900">
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
          <motion.div
            initial={{ opacity: 0 }}
            whileInView={{ opacity: 1 }}
            transition={{ duration: 0.8 }}
            className="text-center mb-16"
          >
            <h2 className="text-4xl md:text-5xl font-bold text-slate-900 dark:text-white mb-4">
              Powerful Features for Modern APIs
            </h2>
            <p className="text-xl text-slate-600 dark:text-slate-400 max-w-2xl mx-auto">
              Everything you need to monitor, detect, and respond to API issues
            </p>
          </motion.div>

          <div className="grid md:grid-cols-2 lg:grid-cols-3 gap-8">
            {features.map((feature, idx) => (
              <motion.div
                key={idx}
                initial={{ opacity: 0, y: 20 }}
                whileInView={{ opacity: 1, y: 0 }}
                transition={{ duration: 0.5, delay: idx * 0.1 }}
                className="p-8 rounded-xl border border-slate-200 dark:border-slate-800 hover:border-blue-400 dark:hover:border-blue-400 transition-colors group"
              >
                <div className="w-12 h-12 rounded-lg bg-blue-100 dark:bg-blue-900/30 flex items-center justify-center text-blue-600 dark:text-blue-400 mb-4 group-hover:scale-110 transition-transform">
                  {feature.icon}
                </div>
                <h3 className="text-lg font-semibold text-slate-900 dark:text-white mb-2">
                  {feature.title}
                </h3>
                <p className="text-slate-600 dark:text-slate-400">
                  {feature.description}
                </p>
              </motion.div>
            ))}
          </div>
        </div>
      </section>

      {/* CTA Section */}
      <section className="py-20 md:py-32 bg-gradient-to-br from-blue-600 to-cyan-600">
        <div className="max-w-4xl mx-auto px-4 sm:px-6 lg:px-8 text-center">
          <motion.div
            initial={{ opacity: 0, scale: 0.95 }}
            whileInView={{ opacity: 1, scale: 1 }}
            transition={{ duration: 0.8 }}
          >
            <h2 className="text-4xl md:text-5xl font-bold text-white mb-6">
              Ready to Transform Your API Monitoring?
            </h2>
            <p className="text-lg text-white/90 mb-8 max-w-2xl mx-auto">
              Join hundreds of companies that trust FaultIQ for intelligent API incident detection and response.
            </p>
            <Link href="/login">
              <Button
                size="lg"
                variant="outline"
                className="bg-white text-blue-600 hover:bg-slate-100 border-0"
              >
                Start Free Trial <ArrowRight className="w-4 h-4 ml-2" />
              </Button>
            </Link>
          </motion.div>
        </div>
      </section>
    </PublicLayout>
  )
}
