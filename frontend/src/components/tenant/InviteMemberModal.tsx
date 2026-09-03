'use client'

import React, { useState } from 'react'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'

interface ProjectOption {
  id: string
  name: string
}

interface Props {
  open: boolean
  onClose: () => void
  onInvite: (member: { name: string; email: string; role: string; projects: string[]; environments: string[] }) => void
  projects: ProjectOption[]
}

export default function InviteMemberModal({ open, onClose, onInvite, projects }: Props) {
  const [name, setName] = useState('')
  const [email, setEmail] = useState('')
  const [role, setRole] = useState('user')
  const [selectedProjects, setSelectedProjects] = useState<string[]>([])
  const [environments, setEnvironments] = useState<string[]>(['dev'])

  if (!open) return null

  function toggleProject(id: string) {
    setSelectedProjects((prev) => (prev.includes(id) ? prev.filter((p) => p !== id) : [...prev, id]))
  }

  function toggleEnvironment(env: string) {
    setEnvironments((prev) => (prev.includes(env) ? prev.filter((e) => e !== env) : [...prev, env]))
  }

  function submit() {
    if (!email) return
    onInvite({ name, email, role, projects: selectedProjects, environments })
    setName('')
    setEmail('')
    setRole('user')
    setSelectedProjects([])
    setEnvironments(['dev'])
    onClose()
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center">
      <div className="absolute inset-0 bg-black/40" onClick={onClose} />
      <Card className="z-20 p-6 w-full max-w-2xl">
        <h3 className="text-lg font-semibold mb-4">Invite Tenant Member</h3>
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          <div>
            <label className="block text-sm font-medium mb-1">Full name</label>
            <input className="w-full px-3 py-2 rounded border" value={name} onChange={(e) => setName(e.target.value)} />
          </div>
          <div>
            <label className="block text-sm font-medium mb-1">Email</label>
            <input className="w-full px-3 py-2 rounded border" value={email} onChange={(e) => setEmail(e.target.value)} />
          </div>
          <div>
            <label className="block text-sm font-medium mb-1">Role</label>
            <select className="w-full px-3 py-2 rounded border" value={role} onChange={(e) => setRole(e.target.value)}>
              <option value="user">User</option>
              <option value="admin">Admin</option>
              <option value="analyst">Analyst</option>
              <option value="viewer">Viewer</option>
            </select>
          </div>
          <div>
            <label className="block text-sm font-medium mb-1">Environments</label>
            <div className="flex gap-2">
              {['dev', 'stage', 'prod'].map((env) => (
                <label key={env} className="inline-flex items-center gap-2">
                  <input type="checkbox" checked={environments.includes(env)} onChange={() => toggleEnvironment(env)} />
                  <span className="capitalize text-sm">{env}</span>
                </label>
              ))}
            </div>
          </div>
        </div>

        <div className="mt-4">
          <label className="block text-sm font-medium mb-2">Assign Projects</label>
          <div className="grid md:grid-cols-2 gap-2 max-h-40 overflow-auto">
            {projects?.map((p) => (
              <label key={p.id} className="inline-flex items-center gap-2 border p-2 rounded">
                <input type="checkbox" checked={selectedProjects.includes(p.id)} onChange={() => toggleProject(p.id)} />
                <span className="text-sm">{p.name}</span>
              </label>
            ))}
          </div>
        </div>

        <div className="flex justify-end gap-2 mt-6">
          <Button variant="ghost" onClick={onClose}>Cancel</Button>
          <Button onClick={submit}>Invite</Button>
        </div>
      </Card>
    </div>
  )
}
