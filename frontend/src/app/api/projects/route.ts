import { NextResponse } from 'next/server'
import { promises as fs } from 'fs'
import { resolve } from 'path'

const PROJECTS_FILE = resolve(process.cwd(), 'src/data/projects.json')

export async function GET() {
  try {
    const raw = await fs.readFile(PROJECTS_FILE, 'utf-8')
    const data = JSON.parse(raw)
    return NextResponse.json(data)
  } catch (e: any) {
    return NextResponse.json([], { status: 200 })
  }
}

export async function POST(req: Request) {
  try {
    const body = await req.json()
    const raw = await fs.readFile(PROJECTS_FILE, 'utf-8')
    const data = JSON.parse(raw)
    const id = body.id || `proj-${Date.now()}`
    const project = { id, name: body.name, owner: body.owner || null, createdAt: new Date().toISOString() }
    data.push(project)
    await fs.writeFile(PROJECTS_FILE, JSON.stringify(data, null, 2), 'utf-8')
    return NextResponse.json(project)
  } catch (e: any) {
    return NextResponse.json({ error: e.message || String(e) }, { status: 500 })
  }
}
