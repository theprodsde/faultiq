"use client"

import { PublicLayout } from "@/components/layouts/PublicLayout"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { useForm } from "react-hook-form"
import { motion } from "framer-motion"

interface ContactFormData {
  name: string
  email: string
  company?: string
  message: string
}

export default function Contact() {
  const { register, handleSubmit, reset } = useForm<ContactFormData>()

  const onSubmit = (data: ContactFormData) => {
    console.log("Contact form submission:", data)
    // TODO: Send to backend
    reset()
  }

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
            <h1 className="text-5xl md:text-6xl font-bold mb-6">Contact Us</h1>
            <p className="text-xl text-slate-300">
              Have questions? We'd love to hear from you.
            </p>
          </motion.div>
        </div>
      </section>

      {/* Contact Section */}
      <section className="py-20 md:py-32 bg-white dark:bg-slate-900/50">
        <div className="max-w-3xl mx-auto px-4 sm:px-6 lg:px-8">
          <div className="grid md:grid-cols-3 gap-8 mb-16">
            {[
              { title: "Email", value: "karan.gehlod@jci.com" },
              { title: "Phone", value: "+91 9669921911" },
              { title: "Address", value: "JCI Banglore Block D" },
            ].map((contact, idx) => (
              <motion.div
                key={idx}
                initial={{ opacity: 0, y: 20 }}
                whileInView={{ opacity: 1, y: 0 }}
                transition={{ duration: 0.5, delay: idx * 0.1 }}
                className="text-center"
              >
                <h3 className="font-semibold text-slate-900 dark:text-white mb-2">
                  {contact.title}
                </h3>
                <p className="text-slate-600 dark:text-slate-400">
                  {contact.value}
                </p>
              </motion.div>
            ))}
          </div>

          {/* Contact Form */}
          <motion.form
            initial={{ opacity: 0, y: 20 }}
            whileInView={{ opacity: 1, y: 0 }}
            transition={{ duration: 0.8 }}
            onSubmit={handleSubmit(onSubmit)}
            className="bg-white dark:bg-slate-900 p-8 rounded-xl border border-slate-200 dark:border-slate-800"
          >
            <div className="grid md:grid-cols-2 gap-6 mb-6">
              <div>
                <Label htmlFor="name" className="mb-2 block">
                  Full Name
                </Label>
                <Input
                  id="name"
                  placeholder="John Doe"
                  {...register("name", { required: true })}
                />
              </div>
              <div>
                <Label htmlFor="email" className="mb-2 block">
                  Email
                </Label>
                <Input
                  id="email"
                  type="email"
                  placeholder="john@example.com"
                  {...register("email", { required: true })}
                />
              </div>
            </div>
            <div className="mb-6">
              <Label htmlFor="company" className="mb-2 block">
                Company
              </Label>
              <Input
                id="company"
                placeholder="Your Company"
                {...register("company")}
              />
            </div>
            <div className="mb-6">
              <Label htmlFor="message" className="mb-2 block">
                Message
              </Label>
              <textarea
                id="message"
                placeholder="Your message here..."
                rows={6}
                className="w-full px-3 py-2 border border-slate-200 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100 dark:placeholder:text-slate-500"
                {...register("message", { required: true })}
              />
            </div>
            <Button type="submit" size="lg" className="w-full">
              Send Message
            </Button>
          </motion.form>
        </div>
      </section>
    </PublicLayout>
  )
}
