import { useEffect, useMemo, useRef, useState, type PointerEvent } from "react";

type EditorMode = "select" | "connect" | "pan";

type ResourceTemplate = {
  id: string;
  type: string;
  title: string;
  color: string;
};

type EditorNode = {
  id: string;
  type: string;
  title: string;
  x: number;
  y: number;
  width: number;
  height: number;
  region: string;
  state: string;
  tags: Record<string, string>;
};

type EditorEdge = {
  id: string;
  from: string;
  to: string;
  type: string;
};

type GraphResponse = {
  schema_version: string;
  generated_at: string;
  nodes: Array<{
    id: string;
    type: string;
    name?: string;
    region?: string;
    state?: string;
    tags?: Record<string, string>;
  }>;
  edges: Array<{
    from: string;
    to: string;
    type: string;
  }>;
};

const STORAGE_KEY = "arch.frontend.canvas.v1";
const CANVAS_WIDTH = 4200;
const CANVAS_HEIGHT = 2600;
const NODE_WIDTH = 190;
const NODE_HEIGHT = 88;

const RESOURCE_LIBRARY: ResourceTemplate[] = [
  { id: "ec2", type: "aws.ec2.instance", title: "EC2 Instance", color: "#f97316" },
  { id: "ecs", type: "aws.ecs.service", title: "ECS Service", color: "#0ea5e9" },
  { id: "alb", type: "aws.elbv2.load_balancer", title: "ALB", color: "#22c55e" },
  { id: "rds", type: "aws.rds.instance", title: "RDS", color: "#f59e0b" },
  { id: "lambda", type: "aws.lambda.function", title: "Lambda", color: "#ef4444" },
  { id: "apigw", type: "aws.apigateway.rest_api", title: "API Gateway", color: "#14b8a6" },
  { id: "s3", type: "aws.s3.bucket", title: "S3 Bucket", color: "#2563eb" },
  { id: "dynamo", type: "aws.dynamodb.table", title: "DynamoDB", color: "#8b5cf6" },
  { id: "sqs", type: "aws.sqs.queue", title: "SQS Queue", color: "#a855f7" },
  { id: "vpc", type: "aws.vpc", title: "VPC", color: "#334155" }
];

function uid(prefix: string): string {
  return `${prefix}-${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 8)}`;
}

function clamp(value: number, min: number, max: number): number {
  return Math.max(min, Math.min(value, max));
}

function centerOf(node: EditorNode): { x: number; y: number } {
  return { x: node.x + node.width / 2, y: node.y + node.height / 2 };
}

function sanitizeNodePosition(x: number, y: number): { x: number; y: number } {
  return {
    x: clamp(x, 24, CANVAS_WIDTH - NODE_WIDTH - 24),
    y: clamp(y, 24, CANVAS_HEIGHT - NODE_HEIGHT - 24)
  };
}

function typeColor(type: string): string {
  const hit = RESOURCE_LIBRARY.find((entry) => type.includes(entry.type.split(".")[1]));
  return hit?.color ?? "#0f766e";
}

function createNode(template: ResourceTemplate, x: number, y: number): EditorNode {
  const pos = sanitizeNodePosition(x, y);
  return {
    id: uid("node"),
    type: template.type,
    title: template.title,
    x: pos.x,
    y: pos.y,
    width: NODE_WIDTH,
    height: NODE_HEIGHT,
    region: "us-east-1",
    state: "active",
    tags: {
      "gocools:stack-id": "dev-stack",
      "gocools:environment": "dev",
      "gocools:owner": "platform-team"
    }
  };
}

function mapGraphToCanvas(payload: GraphResponse): { nodes: EditorNode[]; edges: EditorEdge[] } {
  const columns = 4;
  const nodes = payload.nodes.map((node, index) => {
    const col = index % columns;
    const row = Math.floor(index / columns);
    return {
      id: node.id,
      type: node.type,
      title: node.name || node.id,
      x: 120 + col * 280,
      y: 120 + row * 170,
      width: NODE_WIDTH,
      height: NODE_HEIGHT,
      region: node.region || "us-east-1",
      state: node.state || "active",
      tags: {
        "gocools:stack-id": node.tags?.["gocools:stack-id"] || "",
        "gocools:environment": node.tags?.["gocools:environment"] || "",
        "gocools:owner": node.tags?.["gocools:owner"] || ""
      }
    } satisfies EditorNode;
  });

  const nodeSet = new Set(nodes.map((node) => node.id));
  const edges = payload.edges
    .filter((edge) => nodeSet.has(edge.from) && nodeSet.has(edge.to))
    .map((edge) => ({
      id: uid("edge"),
      from: edge.from,
      to: edge.to,
      type: edge.type || "depends_on"
    }));

  return { nodes, edges };
}

function loadSaved(): { nodes: EditorNode[]; edges: EditorEdge[] } {
  if (typeof window === "undefined") {
    return { nodes: [], edges: [] };
  }

  const raw = window.localStorage.getItem(STORAGE_KEY);
  if (!raw) {
    return { nodes: [], edges: [] };
  }

  try {
    const parsed = JSON.parse(raw) as { nodes?: EditorNode[]; edges?: EditorEdge[] };
    return {
      nodes: Array.isArray(parsed.nodes) ? parsed.nodes : [],
      edges: Array.isArray(parsed.edges) ? parsed.edges : []
    };
  } catch {
    return { nodes: [], edges: [] };
  }
}

type DragState = {
  nodeId: string;
  offsetX: number;
  offsetY: number;
};

type PanState = {
  startClientX: number;
  startClientY: number;
  baseX: number;
  baseY: number;
};

export default function App() {
  const loaded = useMemo(() => loadSaved(), []);
  const [nodes, setNodes] = useState<EditorNode[]>(loaded.nodes);
  const [edges, setEdges] = useState<EditorEdge[]>(loaded.edges);
  const [mode, setMode] = useState<EditorMode>("select");
  const [selectedNodeId, setSelectedNodeId] = useState<string | null>(null);
  const [connectFromId, setConnectFromId] = useState<string | null>(null);
  const [zoom, setZoom] = useState<number>(0.95);
  const [pan, setPan] = useState({ x: 120, y: 80 });
  const [stackFilter, setStackFilter] = useState("dev-stack");
  const [environmentFilter, setEnvironmentFilter] = useState("dev");
  const [status, setStatus] = useState("Canvas ready.");

  const canvasRef = useRef<HTMLDivElement | null>(null);
  const fileInputRef = useRef<HTMLInputElement | null>(null);
  const dragStateRef = useRef<DragState | null>(null);
  const panStateRef = useRef<PanState | null>(null);

  const selectedNode = useMemo(
    () => nodes.find((node) => node.id === selectedNodeId) || null,
    [nodes, selectedNodeId]
  );

  useEffect(() => {
    if (typeof window === "undefined") {
      return;
    }

    window.localStorage.setItem(
      STORAGE_KEY,
      JSON.stringify({
        nodes,
        edges
      })
    );
  }, [nodes, edges]);

  useEffect(() => {
    if (connectFromId && !nodes.some((node) => node.id === connectFromId)) {
      setConnectFromId(null);
    }
  }, [connectFromId, nodes]);

  function worldPoint(clientX: number, clientY: number): { x: number; y: number } {
    const canvas = canvasRef.current;
    if (!canvas) {
      return { x: 0, y: 0 };
    }

    const rect = canvas.getBoundingClientRect();
    return {
      x: (clientX - rect.left - pan.x) / zoom,
      y: (clientY - rect.top - pan.y) / zoom
    };
  }

  function addPaletteNode(template: ResourceTemplate): void {
    const point = worldPoint(window.innerWidth / 2, window.innerHeight / 2);
    setNodes((current) => [...current, createNode(template, point.x - NODE_WIDTH / 2, point.y - NODE_HEIGHT / 2)]);
    setStatus(`Added ${template.title}.`);
  }

  function onPaletteDragStart(event: React.DragEvent<HTMLButtonElement>, template: ResourceTemplate): void {
    event.dataTransfer.setData("application/x-arch-template", template.id);
    event.dataTransfer.effectAllowed = "copy";
  }

  function onCanvasDrop(event: React.DragEvent<HTMLDivElement>): void {
    event.preventDefault();
    const templateId = event.dataTransfer.getData("application/x-arch-template");
    const template = RESOURCE_LIBRARY.find((entry) => entry.id === templateId);
    if (!template) {
      return;
    }

    const point = worldPoint(event.clientX, event.clientY);
    setNodes((current) => [...current, createNode(template, point.x - NODE_WIDTH / 2, point.y - NODE_HEIGHT / 2)]);
    setStatus(`Dropped ${template.title} on canvas.`);
  }

  function onCanvasPointerDown(event: PointerEvent<HTMLDivElement>): void {
    if (event.button === 1 || mode === "pan") {
      panStateRef.current = {
        startClientX: event.clientX,
        startClientY: event.clientY,
        baseX: pan.x,
        baseY: pan.y
      };
      setStatus("Panning canvas.");
      return;
    }

    setSelectedNodeId(null);
    if (mode === "connect") {
      setConnectFromId(null);
    }
  }

  function onCanvasPointerMove(event: PointerEvent<HTMLDivElement>): void {
    if (panStateRef.current) {
      const deltaX = event.clientX - panStateRef.current.startClientX;
      const deltaY = event.clientY - panStateRef.current.startClientY;
      setPan({ x: panStateRef.current.baseX + deltaX, y: panStateRef.current.baseY + deltaY });
      return;
    }

    if (!dragStateRef.current) {
      return;
    }

    const point = worldPoint(event.clientX, event.clientY);
    const next = sanitizeNodePosition(point.x + dragStateRef.current.offsetX, point.y + dragStateRef.current.offsetY);

    setNodes((current) =>
      current.map((node) =>
        node.id === dragStateRef.current?.nodeId
          ? {
              ...node,
              x: next.x,
              y: next.y
            }
          : node
      )
    );
  }

  function onCanvasPointerUp(): void {
    panStateRef.current = null;
    dragStateRef.current = null;
  }

  function onNodePointerDown(event: PointerEvent<HTMLButtonElement>, node: EditorNode): void {
    event.stopPropagation();

    if (mode === "connect") {
      if (!connectFromId) {
        setConnectFromId(node.id);
        setStatus(`Select another node to connect from ${node.title}.`);
        return;
      }

      if (connectFromId === node.id) {
        setConnectFromId(null);
        setStatus("Connection cancelled.");
        return;
      }

      setEdges((current) => {
        const existing = current.some((edge) => edge.from === connectFromId && edge.to === node.id);
        if (existing) {
          return current;
        }
        return [
          ...current,
          {
            id: uid("edge"),
            from: connectFromId,
            to: node.id,
            type: "depends_on"
          }
        ];
      });
      setConnectFromId(null);
      setStatus("Connection created.");
      return;
    }

    setSelectedNodeId(node.id);
    const point = worldPoint(event.clientX, event.clientY);
    dragStateRef.current = {
      nodeId: node.id,
      offsetX: node.x - point.x,
      offsetY: node.y - point.y
    };
  }

  function onCanvasWheel(event: React.WheelEvent<HTMLDivElement>): void {
    if (!event.ctrlKey && !event.metaKey) {
      return;
    }

    event.preventDefault();
    const canvas = canvasRef.current;
    if (!canvas) {
      return;
    }

    const rect = canvas.getBoundingClientRect();
    const pointerX = event.clientX - rect.left;
    const pointerY = event.clientY - rect.top;

    const worldX = (pointerX - pan.x) / zoom;
    const worldY = (pointerY - pan.y) / zoom;

    const delta = event.deltaY > 0 ? -0.08 : 0.08;
    const nextZoom = clamp(zoom + delta, 0.35, 2.4);

    setZoom(nextZoom);
    setPan({
      x: pointerX - worldX * nextZoom,
      y: pointerY - worldY * nextZoom
    });
  }

  function removeSelectedNode(): void {
    if (!selectedNodeId) {
      return;
    }

    setNodes((current) => current.filter((node) => node.id !== selectedNodeId));
    setEdges((current) => current.filter((edge) => edge.from !== selectedNodeId && edge.to !== selectedNodeId));
    setSelectedNodeId(null);
    setStatus("Node deleted.");
  }

  function clearCanvas(): void {
    setNodes([]);
    setEdges([]);
    setSelectedNodeId(null);
    setConnectFromId(null);
    setStatus("Canvas reset.");
  }

  async function loadLiveGraph(): Promise<void> {
    try {
      const query = new URLSearchParams();
      if (stackFilter.trim() !== "") {
        query.set("stack_id", stackFilter.trim());
      }
      if (environmentFilter.trim() !== "") {
        query.set("environment", environmentFilter.trim());
      }

      const response = await fetch(`/api/v1/graph?${query.toString()}`);
      if (!response.ok) {
        throw new Error(`graph load failed with status ${response.status}`);
      }

      const payload = (await response.json()) as GraphResponse;
      const mapped = mapGraphToCanvas(payload);
      setNodes(mapped.nodes);
      setEdges(mapped.edges);
      setSelectedNodeId(null);
      setConnectFromId(null);
      setStatus(`Loaded ${mapped.nodes.length} nodes from Arch API.`);
    } catch (error) {
      const message = error instanceof Error ? error.message : String(error);
      setStatus(`Failed to load graph: ${message}`);
    }
  }

  function exportJson(): void {
    const payload = {
      schema_version: "arch.frontend/v1alpha1",
      exported_at: new Date().toISOString(),
      nodes,
      edges,
      viewport: { zoom, pan }
    };

    const blob = new Blob([JSON.stringify(payload, null, 2)], { type: "application/json" });
    const url = URL.createObjectURL(blob);
    const anchor = document.createElement("a");
    anchor.href = url;
    anchor.download = `arch-diagram-${Date.now()}.json`;
    anchor.click();
    URL.revokeObjectURL(url);
    setStatus("Diagram exported.");
  }

  function openImportDialog(): void {
    fileInputRef.current?.click();
  }

  function onImportFile(event: React.ChangeEvent<HTMLInputElement>): void {
    const file = event.target.files?.[0];
    if (!file) {
      return;
    }

    const reader = new FileReader();
    reader.onload = () => {
      try {
        const parsed = JSON.parse(String(reader.result)) as { nodes?: EditorNode[]; edges?: EditorEdge[] };
        if (!Array.isArray(parsed.nodes) || !Array.isArray(parsed.edges)) {
          throw new Error("invalid diagram payload");
        }

        setNodes(parsed.nodes);
        setEdges(parsed.edges);
        setSelectedNodeId(null);
        setConnectFromId(null);
        setStatus(`Imported ${parsed.nodes.length} nodes.`);
      } catch (error) {
        const message = error instanceof Error ? error.message : String(error);
        setStatus(`Import failed: ${message}`);
      }
    };
    reader.readAsText(file);

    event.target.value = "";
  }

  function updateSelectedNode(patch: Partial<EditorNode>): void {
    if (!selectedNodeId) {
      return;
    }

    setNodes((current) =>
      current.map((node) =>
        node.id === selectedNodeId
          ? {
              ...node,
              ...patch
            }
          : node
      )
    );
  }

  return (
    <div className="arch-root">
      <header className="topbar">
        <div>
          <h1>Arch Canvas</h1>
          <p>AWS architecture editor for Arch.gocools</p>
        </div>
        <div className="topbar-actions">
          <button onClick={() => setZoom((current) => clamp(current - 0.1, 0.35, 2.4))}>-</button>
          <span>{Math.round(zoom * 100)}%</span>
          <button onClick={() => setZoom((current) => clamp(current + 0.1, 0.35, 2.4))}>+</button>
          <button onClick={clearCanvas}>New</button>
          <button onClick={openImportDialog}>Import</button>
          <button onClick={exportJson}>Export</button>
          <button onClick={loadLiveGraph}>Load Live Graph</button>
          <input ref={fileInputRef} type="file" accept="application/json" onChange={onImportFile} hidden />
        </div>
      </header>

      <div className="status-row">{status}</div>

      <div className="workspace-grid">
        <aside className="panel toolbox">
          <h2>AWS Palette</h2>
          <p>Drag components onto the canvas.</p>
          <div className="resource-list">
            {RESOURCE_LIBRARY.map((template) => (
              <button
                key={template.id}
                className="resource-item"
                style={{ borderColor: template.color }}
                draggable
                onDragStart={(event) => onPaletteDragStart(event, template)}
                onClick={() => addPaletteNode(template)}
              >
                <span className="dot" style={{ backgroundColor: template.color }} />
                <span>{template.title}</span>
              </button>
            ))}
          </div>

          <h3>Mode</h3>
          <div className="mode-grid">
            <button className={mode === "select" ? "mode active" : "mode"} onClick={() => setMode("select")}>
              Select
            </button>
            <button className={mode === "connect" ? "mode active" : "mode"} onClick={() => setMode("connect")}>
              Connect
            </button>
            <button className={mode === "pan" ? "mode active" : "mode"} onClick={() => setMode("pan")}>
              Pan
            </button>
          </div>

          <h3>Graph Filter</h3>
          <label>
            Stack ID
            <input value={stackFilter} onChange={(event) => setStackFilter(event.target.value)} />
          </label>
          <label>
            Environment
            <input value={environmentFilter} onChange={(event) => setEnvironmentFilter(event.target.value)} />
          </label>
        </aside>

        <section className="canvas-column">
          <div
            className="canvas-shell"
            ref={canvasRef}
            onDrop={onCanvasDrop}
            onDragOver={(event) => event.preventDefault()}
            onPointerDown={onCanvasPointerDown}
            onPointerMove={onCanvasPointerMove}
            onPointerUp={onCanvasPointerUp}
            onPointerLeave={onCanvasPointerUp}
            onWheel={onCanvasWheel}
            onContextMenu={(event) => event.preventDefault()}
          >
            <div
              className="canvas-stage"
              style={{
                width: `${CANVAS_WIDTH}px`,
                height: `${CANVAS_HEIGHT}px`,
                transform: `translate(${pan.x}px, ${pan.y}px) scale(${zoom})`
              }}
            >
              <svg className="edges" width={CANVAS_WIDTH} height={CANVAS_HEIGHT}>
                <defs>
                  <marker id="arrow" markerWidth="8" markerHeight="8" refX="7" refY="4" orient="auto">
                    <path d="M0,0 L8,4 L0,8 Z" fill="#0f766e" />
                  </marker>
                </defs>
                {edges.map((edge) => {
                  const from = nodes.find((node) => node.id === edge.from);
                  const to = nodes.find((node) => node.id === edge.to);
                  if (!from || !to) {
                    return null;
                  }

                  const start = centerOf(from);
                  const end = centerOf(to);
                  const controlX = (start.x + end.x) / 2;
                  const path = `M ${start.x} ${start.y} C ${controlX} ${start.y}, ${controlX} ${end.y}, ${end.x} ${end.y}`;

                  return <path key={edge.id} d={path} className="edge-path" markerEnd="url(#arrow)" />;
                })}
              </svg>

              {nodes.map((node) => (
                <button
                  key={node.id}
                  className={
                    node.id === selectedNodeId
                      ? "canvas-node selected"
                      : node.id === connectFromId
                        ? "canvas-node connecting"
                        : "canvas-node"
                  }
                  style={{
                    left: node.x,
                    top: node.y,
                    width: node.width,
                    height: node.height,
                    borderColor: typeColor(node.type)
                  }}
                  onPointerDown={(event) => onNodePointerDown(event, node)}
                >
                  <span className="node-type">{node.type}</span>
                  <strong>{node.title}</strong>
                  <small>
                    {node.region} - {node.state}
                  </small>
                </button>
              ))}
            </div>
          </div>
        </section>

        <aside className="panel inspector">
          <h2>Inspector</h2>
          {!selectedNode ? (
            <p>Select a node to edit details.</p>
          ) : (
            <>
              <label>
                Title
                <input
                  value={selectedNode.title}
                  onChange={(event) => updateSelectedNode({ title: event.target.value })}
                />
              </label>
              <label>
                Type
                <input
                  value={selectedNode.type}
                  onChange={(event) => updateSelectedNode({ type: event.target.value })}
                />
              </label>
              <label>
                Region
                <input
                  value={selectedNode.region}
                  onChange={(event) => updateSelectedNode({ region: event.target.value })}
                />
              </label>
              <label>
                State
                <input
                  value={selectedNode.state}
                  onChange={(event) => updateSelectedNode({ state: event.target.value })}
                />
              </label>
              <label>
                Stack Tag
                <input
                  value={selectedNode.tags["gocools:stack-id"] || ""}
                  onChange={(event) =>
                    updateSelectedNode({
                      tags: {
                        ...selectedNode.tags,
                        "gocools:stack-id": event.target.value
                      }
                    })
                  }
                />
              </label>
              <label>
                Environment Tag
                <input
                  value={selectedNode.tags["gocools:environment"] || ""}
                  onChange={(event) =>
                    updateSelectedNode({
                      tags: {
                        ...selectedNode.tags,
                        "gocools:environment": event.target.value
                      }
                    })
                  }
                />
              </label>
              <label>
                Owner Tag
                <input
                  value={selectedNode.tags["gocools:owner"] || ""}
                  onChange={(event) =>
                    updateSelectedNode({
                      tags: {
                        ...selectedNode.tags,
                        "gocools:owner": event.target.value
                      }
                    })
                  }
                />
              </label>
              <button className="danger" onClick={removeSelectedNode}>
                Delete Node
              </button>
            </>
          )}

          <div className="meta-block">
            <h3>Canvas Stats</h3>
            <p>Nodes: {nodes.length}</p>
            <p>Edges: {edges.length}</p>
            <p>Mode: {mode}</p>
            {connectFromId ? <p>Connecting from: {connectFromId}</p> : null}
          </div>
          <p className="powered-by">Interaction model inspired by edit.gocools.</p>
        </aside>
      </div>
    </div>
  );
}
