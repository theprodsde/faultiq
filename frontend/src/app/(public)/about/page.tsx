"use client"

import { PublicLayout } from "@/components/layouts/PublicLayout"
import { Button } from "@/components/ui/button"
import Link from "next/link"
import { motion } from "framer-motion"

export default function About() {
  return (
    <PublicLayout>
      {/* Hero */}
      <section className="py-20 md:py-32 bg-gradient-to-br from-slate-900 to-slate-800 dark:from-slate-950 dark:to-slate-900 text-white">
        <div className="max-w-4xl mx-auto px-4 sm:px-6 lg:px-8">
          <motion.div
            initial={{ opacity: 0, y: 20 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ duration: 0.8 }}
          >
            <h1 className="text-5xl md:text-6xl font-bold mb-6">About FaultIQ</h1>
            <p className="text-xl text-slate-300 max-w-2xl">
              Empowering enterprises with intelligent API monitoring and incident detection.
            </p>
          </motion.div>
        </div>
      </section>

      {/* Mission Section */}
      <section className="py-20 md:py-32 bg-white dark:bg-slate-900/50">
        <div className="max-w-4xl mx-auto px-4 sm:px-6 lg:px-8">
          <motion.div
            initial={{ opacity: 0 }}
            whileInView={{ opacity: 1 }}
            transition={{ duration: 0.8 }}
          >
            <h2 className="text-4xl font-bold text-slate-900 dark:text-white mb-8">Our Mission</h2>
            <p className="text-lg text-slate-600 dark:text-slate-400 mb-6">
              FaultIQ was founded with a simple mission: to help enterprises detect and resolve API failures before they impact their users. We believe that reliable APIs are the foundation of modern software, and intelligent monitoring is the key to maintaining that reliability.
            </p>
            <p className="text-lg text-slate-600 dark:text-slate-400">
              By combining advanced machine learning, graph analytics, and real-time data processing, we've built a platform that doesn't just alert you to problems—it helps you understand and fix them faster.
            </p>
          </motion.div>
        </div>
      </section>

      {/* Values Section */}
      <section className="py-20 md:py-32 bg-slate-50 dark:bg-slate-900">
        <div className="max-w-6xl mx-auto px-4 sm:px-6 lg:px-8">
          <h2 className="text-4xl font-bold text-slate-900 dark:text-white mb-16 text-center">Our Values</h2>
          <div className="grid md:grid-cols-3 gap-8">
            {[
              { title: "Reliability", desc: "We believe in building systems you can trust." },
              { title: "Innovation", desc: "We push boundaries with AI and advanced analytics." },
              { title: "Transparency", desc: "We provide clear insights into what matters." },
            ].map((value, idx) => (
              <motion.div
                key={idx}
                initial={{ opacity: 0, y: 20 }}
                whileInView={{ opacity: 1, y: 0 }}
                transition={{ duration: 0.5, delay: idx * 0.1 }}
                className="p-8 rounded-xl bg-white dark:bg-slate-800 border border-slate-200 dark:border-slate-700"
              >
                <h3 className="text-2xl font-bold text-slate-900 dark:text-white mb-3">
                  {value.title}
                </h3>
                <p className="text-slate-600 dark:text-slate-400">
                  {value.desc}
                </p>
              </motion.div>
            ))}
          </div>
        </div>
      </section>

      {/* CTA */}
      <section className="py-20 md:py-32 text-center bg-white dark:bg-slate-900/50">
        <div className="max-w-3xl mx-auto px-4 sm:px-6 lg:px-8">
          <h2 className="text-4xl font-bold text-slate-900 dark:text-white mb-6">
            Join Our Community
          </h2>
          <p className="text-lg text-slate-600 dark:text-slate-400 mb-8">
            Start monitoring your APIs intelligently today.
          </p>
          <Link href="/login">
            <Button size="lg">Get Started</Button>
          </Link>
        </div>
      </section>
    </PublicLayout>
  )
}
