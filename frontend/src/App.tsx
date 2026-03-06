import {
  useEffect,
  useMemo,
  useRef,
  useState,
  type ChangeEvent,
  type DragEvent,
  type PointerEvent,
  type WheelEvent
} from "react";

type EditorMode = "select" | "connect" | "pan";
type OperationAction = "create" | "update" | "scale" | "destroy";

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

type FrameKind = "cloud" | "vpc" | "az" | "subnet-public" | "subnet-private";

type EditorFrame = {
  id: string;
  title: string;
  note?: string;
  x: number;
  y: number;
  width: number;
  height: number;
  kind: FrameKind;
};

type GraphNodePayload = {
  id: string;
  type: string;
  provider?: string;
  region?: string;
  name?: string;
  state?: string;
  arn?: string;
  tags?: Record<string, string>;
  metadata?: Record<string, string>;
};

type GraphEdgePayload = {
  from: string;
  to: string;
  type: string;
  metadata?: Record<string, string>;
};

type GraphSnapshot = {
  schema_version: string;
  generated_at: string;
  nodes: GraphNodePayload[];
  edges: GraphEdgePayload[];
};

type DiffFieldChange = {
  field: string;
  before?: string;
  after?: string;
};

type GraphDiffChange = {
  kind: "added" | "removed" | "modified";
  node_id: string;
  resource_type: string;
  changes?: DiffFieldChange[];
};

type GraphDiffReport = {
  added: number;
  removed: number;
  modified: number;
  changes: GraphDiffChange[];
};

type StackOperationRequest = {
  action: OperationAction;
  stack_id: string;
  environment: string;
  actor: string;
  replicas?: number;
  tags?: Record<string, string>;
  metadata?: Record<string, string>;
  confirm?: boolean;
  dry_run?: boolean;
  manual_override?: boolean;
};

type StackOperationResponse = {
  executed: boolean;
  dry_run: boolean;
  message: string;
  stack?: {
    id: string;
    environment: string;
    replicas: number;
    tags?: Record<string, string>;
    metadata?: Record<string, string>;
  };
  audit: {
    timestamp: string;
    actor: string;
    stack_id: string;
    environment: string;
    action: OperationAction;
    dry_run: boolean;
    result: string;
  };
};

type AWSConnectRequest = {
  region: string;
  access_key_id: string;
  secret_access_key: string;
  session_token?: string;
  role_arn?: string;
  session_name?: string;
  external_id?: string;
  stack_id?: string;
  environment?: string;
  validate_on_start?: boolean;
};

type AWSConnectResponse = {
  connected: boolean;
  provider: string;
  region: string;
  identity: {
    account_id?: string;
    arn?: string;
    user_id?: string;
  };
  graph: GraphSnapshot;
};

type ErrorPayload = {
  error?: string;
};

type GuardrailState = {
  blocking: string[];
  warnings: string[];
};

const STORAGE_KEY = "arch.frontend.canvas.v1";
const CANVAS_WIDTH = 4200;
const CANVAS_HEIGHT = 2600;
const NODE_WIDTH = 190;
const NODE_HEIGHT = 88;
const REQUIRED_TAG_KEYS = ["gocools:stack-id", "gocools:environment", "gocools:owner"] as const;

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

const TYPE_COLOR_OVERRIDES: Array<{ prefix: string; color: string }> = [
  { prefix: "aws.vpc", color: "#334155" },
  { prefix: "aws.ec2.subnet", color: "#64748b" },
  { prefix: "aws.ec2.internet_gateway", color: "#f43f5e" },
  { prefix: "aws.ec2.nat_gateway", color: "#f97316" },
  { prefix: "aws.ec2.route_table", color: "#0ea5e9" },
  { prefix: "aws.rds.db_instance", color: "#f59e0b" }
];

const TYPE_ICON_OVERRIDES: Array<{ prefix: string; icon: string }> = [
  { prefix: "aws.vpc", icon: "VPC" },
  { prefix: "aws.ec2.subnet", icon: "SUB" },
  { prefix: "aws.ec2.internet_gateway", icon: "IGW" },
  { prefix: "aws.ec2.nat_gateway", icon: "NAT" },
  { prefix: "aws.ec2.instance", icon: "EC2" },
  { prefix: "aws.ecs.service", icon: "ECS" },
  { prefix: "aws.elbv2.load_balancer", icon: "ALB" },
  { prefix: "aws.rds.db_instance", icon: "RDS" },
  { prefix: "aws.rds.instance", icon: "RDS" },
  { prefix: "aws.lambda.function", icon: "LMB" },
  { prefix: "aws.apigateway.rest_api", icon: "API" },
  { prefix: "aws.s3.bucket", icon: "S3" },
  { prefix: "aws.dynamodb.table", icon: "DDB" },
  { prefix: "aws.sqs.queue", icon: "SQS" }
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

function sanitizeFramePosition(x: number, y: number, width: number, height: number): { x: number; y: number } {
  return {
    x: clamp(x, 12, CANVAS_WIDTH - width - 12),
    y: clamp(y, 12, CANVAS_HEIGHT - height - 12)
  };
}

function normalizeRequiredTags(tags: Record<string, string> | undefined): Record<string, string> {
  const next = {
    ...(tags || {})
  };

  for (const key of REQUIRED_TAG_KEYS) {
    if (!(key in next)) {
      next[key] = "";
    }
  }

  return next;
}

function typeColor(type: string): string {
  const override = TYPE_COLOR_OVERRIDES.find((entry) => type === entry.prefix || type.startsWith(`${entry.prefix}.`));
  if (override) {
    return override.color;
  }
  const hit = RESOURCE_LIBRARY.find((entry) => type === entry.type || type.startsWith(`${entry.type}.`));
  return hit?.color ?? "#0f766e";
}

function typeIcon(type: string): string {
  const override = TYPE_ICON_OVERRIDES.find((entry) => type === entry.prefix || type.startsWith(`${entry.prefix}.`));
  if (override) {
    return override.icon;
  }
  return "AWS";
}

function createTypedNode(
  type: string,
  title: string,
  x: number,
  y: number,
  defaults: { stackId: string; environment: string; owner: string },
  region: string
): EditorNode {
  const pos = sanitizeNodePosition(x, y);
  return {
    id: uid("node"),
    type,
    title,
    x: pos.x,
    y: pos.y,
    width: NODE_WIDTH,
    height: NODE_HEIGHT,
    region,
    state: "active",
    tags: normalizeRequiredTags({
      "gocools:stack-id": defaults.stackId,
      "gocools:environment": defaults.environment,
      "gocools:owner": defaults.owner
    })
  };
}

function createFrame(
  kind: FrameKind,
  title: string,
  note: string | undefined,
  x: number,
  y: number,
  width: number,
  height: number
): EditorFrame {
  const pos = sanitizeFramePosition(x, y, width, height);
  return {
    id: uid("frame"),
    kind,
    title,
    note,
    x: pos.x,
    y: pos.y,
    width,
    height
  };
}

function createNode(
  template: ResourceTemplate,
  x: number,
  y: number,
  defaults: { stackId: string; environment: string; owner: string },
  region: string
): EditorNode {
  return createTypedNode(template.type, template.title, x, y, defaults, region);
}

function buildVPCStarterBlueprint(
  x: number,
  y: number,
  defaults: { stackId: string; environment: string; owner: string },
  region: string
): { frames: EditorFrame[]; nodes: EditorNode[]; edges: EditorEdge[] } {
  const frames: EditorFrame[] = [
    createFrame("cloud", "AWS Cloud", undefined, x - 260, y - 210, 1280, 760),
    createFrame("vpc", "Virtual Private Cloud (10.0.0.0/16)", undefined, x - 220, y - 160, 1200, 660),
    createFrame("az", "Availability Zone A", undefined, x - 185, y - 110, 1120, 270),
    createFrame("az", "Availability Zone B", undefined, x - 185, y + 200, 1120, 260),
    createFrame("subnet-public", "Public Subnet A (10.0.1.0/24)", undefined, x - 160, y - 80, 310, 220),
    createFrame("subnet-private", "Private Subnet A (10.0.11.0/24)", undefined, x + 210, y - 80, 690, 220),
    createFrame("subnet-public", "Public Subnet B (10.0.2.0/24)", undefined, x - 160, y + 225, 310, 210),
    createFrame("subnet-private", "Private Subnet B (10.0.12.0/24)", undefined, x + 210, y + 225, 690, 210)
  ];

  const vpc = createTypedNode("aws.vpc", "Main VPC", x - 15, y + 160, defaults, region);
  const internetGateway = createTypedNode("aws.ec2.internet_gateway", "Internet Gateway", x - 95, y + 18, defaults, region);
  const alb = createTypedNode("aws.elbv2.load_balancer", "Public ALB", x + 265, y + 18, defaults, region);
  const natGatewayA = createTypedNode("aws.ec2.nat_gateway", "NAT Gateway A", x - 95, y + 100, defaults, region);
  const natGatewayB = createTypedNode("aws.ec2.nat_gateway", "NAT Gateway B", x - 95, y + 315, defaults, region);
  const appServiceA = createTypedNode("aws.ecs.service", "App Service A", x + 350, y + 95, defaults, region);
  const appServiceB = createTypedNode("aws.ecs.service", "App Service B", x + 350, y + 315, defaults, region);
  const database = createTypedNode("aws.rds.db_instance", "RDS Database", x + 640, y + 210, defaults, region);
  const eksControlPlane = createTypedNode("aws.eks.cluster", "EKS Control Plane", x + 865, y + 160, defaults, region);

  const nodes = [vpc, internetGateway, alb, natGatewayA, natGatewayB, appServiceA, appServiceB, database, eksControlPlane];
  const edges: EditorEdge[] = [
    { id: uid("edge"), from: internetGateway.id, to: vpc.id, type: "attached_to" },
    { id: uid("edge"), from: alb.id, to: internetGateway.id, type: "ingress_via" },
    { id: uid("edge"), from: natGatewayA.id, to: internetGateway.id, type: "egress_via" },
    { id: uid("edge"), from: natGatewayB.id, to: internetGateway.id, type: "egress_via" },
    { id: uid("edge"), from: appServiceA.id, to: natGatewayA.id, type: "egress_via" },
    { id: uid("edge"), from: appServiceB.id, to: natGatewayB.id, type: "egress_via" },
    { id: uid("edge"), from: alb.id, to: appServiceA.id, type: "routes_to" },
    { id: uid("edge"), from: alb.id, to: appServiceB.id, type: "routes_to" },
    { id: uid("edge"), from: appServiceA.id, to: database.id, type: "depends_on" },
    { id: uid("edge"), from: appServiceB.id, to: database.id, type: "depends_on" },
    { id: uid("edge"), from: appServiceA.id, to: eksControlPlane.id, type: "managed_by" },
    { id: uid("edge"), from: appServiceB.id, to: eksControlPlane.id, type: "managed_by" }
  ];

  return { frames, nodes, edges };
}

function mapGraphToCanvas(payload: GraphSnapshot): { nodes: EditorNode[]; edges: EditorEdge[] } {
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
      tags: normalizeRequiredTags(node.tags)
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

function canvasToGraph(nodes: EditorNode[], edges: EditorEdge[]): GraphSnapshot {
  const graphNodes: GraphNodePayload[] = nodes.map((node) => ({
    id: node.id,
    type: node.type,
    provider: node.type.split(".")[0] || "aws",
    region: node.region,
    name: node.title,
    state: node.state,
    tags: normalizeRequiredTags(node.tags),
    metadata: {
      source: "arch.frontend",
      canvas_x: String(Math.round(node.x)),
      canvas_y: String(Math.round(node.y))
    }
  }));

  const graphEdges: GraphEdgePayload[] = edges.map((edge) => ({
    from: edge.from,
    to: edge.to,
    type: edge.type,
    metadata: {
      source: "arch.frontend"
    }
  }));

  return {
    schema_version: "arch.gocools/v1alpha1",
    generated_at: new Date().toISOString(),
    nodes: graphNodes,
    edges: graphEdges
  };
}

function buildNodeTagGuardrails(nodes: EditorNode[], stackID: string, environment: string): GuardrailState {
  const blocking: string[] = [];
  const warnings: string[] = [];

  for (const node of nodes) {
    const missing = REQUIRED_TAG_KEYS.filter((key) => !node.tags[key] || node.tags[key].trim() === "");
    if (missing.length > 0) {
      blocking.push(`Node ${node.title} missing tags: ${missing.join(", ")}.`);
      continue;
    }

    if (stackID !== "" && node.tags["gocools:stack-id"] !== stackID) {
      warnings.push(
        `Node ${node.title} stack tag (${node.tags["gocools:stack-id"]}) differs from filter (${stackID}).`
      );
    }

    if (environment !== "" && node.tags["gocools:environment"] !== environment) {
      warnings.push(
        `Node ${node.title} environment tag (${node.tags["gocools:environment"]}) differs from filter (${environment}).`
      );
    }
  }

  return { blocking, warnings };
}

function parsePositiveInt(value: string): number | null {
  const parsed = Number.parseInt(value, 10);
  if (!Number.isFinite(parsed) || parsed <= 0) {
    return null;
  }
  return parsed;
}

function summarizeDiff(change: GraphDiffChange): string {
  if (!change.changes || change.changes.length === 0) {
    return "No field-level changes.";
  }

  return change.changes
    .slice(0, 2)
    .map((fieldChange) => {
      if (fieldChange.before !== undefined && fieldChange.after !== undefined) {
        return `${fieldChange.field}: ${fieldChange.before} -> ${fieldChange.after}`;
      }
      if (fieldChange.before !== undefined) {
        return `${fieldChange.field}: removed ${fieldChange.before}`;
      }
      return `${fieldChange.field}: added ${fieldChange.after || ""}`;
    })
    .join("; ");
}

function loadSaved(): { frames: EditorFrame[]; nodes: EditorNode[]; edges: EditorEdge[] } {
  if (typeof window === "undefined") {
    return { frames: [], nodes: [], edges: [] };
  }

  const raw = window.localStorage.getItem(STORAGE_KEY);
  if (!raw) {
    return { frames: [], nodes: [], edges: [] };
  }

  try {
    const parsed = JSON.parse(raw) as { frames?: EditorFrame[]; nodes?: EditorNode[]; edges?: EditorEdge[] };
    return {
      frames: Array.isArray(parsed.frames)
        ? parsed.frames.map((frame) => ({
            ...frame
          }))
        : [],
      nodes: Array.isArray(parsed.nodes)
        ? parsed.nodes.map((node) => ({
            ...node,
            width: node.width || NODE_WIDTH,
            height: node.height || NODE_HEIGHT,
            tags: normalizeRequiredTags(node.tags)
          }))
        : [],
      edges: Array.isArray(parsed.edges) ? parsed.edges : []
    };
  } catch {
    return { frames: [], nodes: [], edges: [] };
  }
}

async function parseApiError(response: Response): Promise<string> {
  try {
    const payload = (await response.json()) as ErrorPayload;
    if (typeof payload.error === "string" && payload.error.trim() !== "") {
      return payload.error;
    }
  } catch {
    // Fall through to default error.
  }

  return `request failed with status ${response.status}`;
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

  const [frames, setFrames] = useState<EditorFrame[]>(loaded.frames);
  const [nodes, setNodes] = useState<EditorNode[]>(loaded.nodes);
  const [edges, setEdges] = useState<EditorEdge[]>(loaded.edges);
  const [mode, setMode] = useState<EditorMode>("select");
  const [selectedNodeId, setSelectedNodeId] = useState<string | null>(null);
  const [connectFromId, setConnectFromId] = useState<string | null>(null);
  const [zoom, setZoom] = useState<number>(0.95);
  const [pan, setPan] = useState({ x: 120, y: 80 });

  const [stackFilter, setStackFilter] = useState("dev-stack");
  const [environmentFilter, setEnvironmentFilter] = useState("dev");
  const [actor, setActor] = useState("platform-team");
  const [operationOwner, setOperationOwner] = useState("platform-team");
  const [operationAction, setOperationAction] = useState<OperationAction>("create");
  const [replicas, setReplicas] = useState("2");
  const [dryRun, setDryRun] = useState(true);
  const [confirmDestroy, setConfirmDestroy] = useState(false);
  const [manualOverride, setManualOverride] = useState(false);
  const [awsRegion, setAwsRegion] = useState("us-east-1");
  const [awsAccessKeyID, setAwsAccessKeyID] = useState("");
  const [awsSecretAccessKey, setAwsSecretAccessKey] = useState("");
  const [awsSessionToken, setAwsSessionToken] = useState("");
  const [awsRoleARN, setAwsRoleARN] = useState("");
  const [awsSessionName, setAwsSessionName] = useState("arch-ui-session");
  const [awsExternalID, setAwsExternalID] = useState("");
  const [awsValidateOnConnect, setAwsValidateOnConnect] = useState(true);
  const [awsApplyTagFilters, setAwsApplyTagFilters] = useState(false);

  const [status, setStatus] = useState("Canvas ready.");
  const [liveGraphSnapshot, setLiveGraphSnapshot] = useState<GraphSnapshot | null>(null);
  const [diffReport, setDiffReport] = useState<GraphDiffReport | null>(null);
  const [lastOperation, setLastOperation] = useState<StackOperationResponse | null>(null);
  const [awsConnection, setAwsConnection] = useState<AWSConnectResponse | null>(null);
  const [isPlanning, setIsPlanning] = useState(false);
  const [isApplying, setIsApplying] = useState(false);
  const [isConnectingAWS, setIsConnectingAWS] = useState(false);

  const canvasRef = useRef<HTMLDivElement | null>(null);
  const fileInputRef = useRef<HTMLInputElement | null>(null);
  const dragStateRef = useRef<DragState | null>(null);
  const panStateRef = useRef<PanState | null>(null);

  const selectedNode = useMemo(
    () => nodes.find((node) => node.id === selectedNodeId) || null,
    [nodes, selectedNodeId]
  );

  const trimmedStack = stackFilter.trim();
  const trimmedEnvironment = environmentFilter.trim();
  const trimmedActor = actor.trim();
  const trimmedOwner = operationOwner.trim();
  const trimmedAWSRegion = awsRegion.trim();
  const trimmedAWSAccessKeyID = awsAccessKeyID.trim();
  const trimmedAWSSecretAccessKey = awsSecretAccessKey.trim();
  const trimmedAWSSessionToken = awsSessionToken.trim();
  const trimmedAWSRoleARN = awsRoleARN.trim();
  const trimmedAWSSessionName = awsSessionName.trim();
  const trimmedAWSExternalID = awsExternalID.trim();

  const nodeTagGuardrails = useMemo(
    () => buildNodeTagGuardrails(nodes, trimmedStack, trimmedEnvironment),
    [nodes, trimmedStack, trimmedEnvironment]
  );

  const operationGuardrails = useMemo<GuardrailState>(() => {
    const blocking: string[] = [];
    const warnings = [...nodeTagGuardrails.warnings];

    if (trimmedStack === "") {
      blocking.push("Stack ID is required.");
    }
    if (trimmedEnvironment === "") {
      blocking.push("Environment is required.");
    }
    if (trimmedActor === "") {
      blocking.push("Actor is required.");
    }

    if (operationAction === "create" || operationAction === "update") {
      if (trimmedOwner === "") {
        blocking.push("Owner tag is required for create/update operations.");
      }
      blocking.push(...nodeTagGuardrails.blocking);
    }

    if (operationAction === "create" || operationAction === "scale") {
      if (parsePositiveInt(replicas) === null) {
        blocking.push("Replicas must be a positive integer for create/scale.");
      }
    }

    if (operationAction === "destroy") {
      if (!confirmDestroy) {
        blocking.push("Destroy requires confirm=true.");
      }
      if (trimmedEnvironment === "prod" && !manualOverride) {
        blocking.push("Destroy in prod requires manual_override=true.");
      }
    }

    if ((operationAction === "create" || operationAction === "update") && nodes.length === 0) {
      warnings.push("Canvas has no nodes; operation will only affect stack metadata.");
    }

    return {
      blocking,
      warnings
    };
  }, [
    confirmDestroy,
    manualOverride,
    nodeTagGuardrails.blocking,
    nodeTagGuardrails.warnings,
    nodes.length,
    operationAction,
    replicas,
    trimmedActor,
    trimmedEnvironment,
    trimmedOwner,
    trimmedStack
  ]);

  useEffect(() => {
    if (typeof window === "undefined") {
      return;
    }

    window.localStorage.setItem(
      STORAGE_KEY,
      JSON.stringify({
        frames,
        nodes,
        edges
      })
    );
  }, [frames, nodes, edges]);

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
    addTemplateToCanvas(template, point.x, point.y, "Added");
  }

  function addTemplateToCanvas(template: ResourceTemplate, worldX: number, worldY: number, action: "Added" | "Dropped"): void {
    const defaults = {
      stackId: trimmedStack || "dev-stack",
      environment: trimmedEnvironment || "dev",
      owner: trimmedOwner || "platform-team"
    };
    const region = trimmedAWSRegion || "us-east-1";
    const x = worldX - NODE_WIDTH / 2;
    const y = worldY - NODE_HEIGHT / 2;

    if (template.id === "vpc") {
      const starter = buildVPCStarterBlueprint(x, y, defaults, region);
      setFrames((current) => [...current, ...starter.frames]);
      setNodes((current) => [...current, ...starter.nodes]);
      setEdges((current) => [...current, ...starter.edges]);
      setStatus(
        `${action} VPC blueprint (${starter.frames.length} frames, ${starter.nodes.length} nodes, ${starter.edges.length} links).`
      );
      return;
    }

    setNodes((current) => [...current, createNode(template, x, y, defaults, region)]);
    setStatus(action === "Added" ? `Added ${template.title}.` : `Dropped ${template.title} on canvas.`);
  }

  function onPaletteDragStart(event: DragEvent<HTMLButtonElement>, template: ResourceTemplate): void {
    event.dataTransfer.setData("application/x-arch-template", template.id);
    event.dataTransfer.effectAllowed = "copy";
  }

  function onCanvasDrop(event: DragEvent<HTMLDivElement>): void {
    event.preventDefault();
    const templateId = event.dataTransfer.getData("application/x-arch-template");
    const template = RESOURCE_LIBRARY.find((entry) => entry.id === templateId);
    if (!template) {
      return;
    }

    const point = worldPoint(event.clientX, event.clientY);
    addTemplateToCanvas(template, point.x, point.y, "Dropped");
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

  function onCanvasWheel(event: WheelEvent<HTMLDivElement>): void {
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
    setFrames([]);
    setNodes([]);
    setEdges([]);
    setSelectedNodeId(null);
    setConnectFromId(null);
    setDiffReport(null);
    setStatus("Canvas reset.");
  }

  async function fetchLiveGraphSnapshot(): Promise<GraphSnapshot> {
    const query = new URLSearchParams();
    if (trimmedStack !== "") {
      query.set("stack_id", trimmedStack);
    }
    if (trimmedEnvironment !== "") {
      query.set("environment", trimmedEnvironment);
    }

    const response = await fetch(`/api/v1/graph?${query.toString()}`);
    if (!response.ok) {
      throw new Error(await parseApiError(response));
    }

    const payload = (await response.json()) as GraphSnapshot;
    return {
      schema_version: payload.schema_version || "arch.gocools/v1alpha1",
      generated_at: payload.generated_at || new Date().toISOString(),
      nodes: Array.isArray(payload.nodes) ? payload.nodes : [],
      edges: Array.isArray(payload.edges) ? payload.edges : []
    };
  }

  async function loadLiveGraph(): Promise<void> {
    try {
      const payload = await fetchLiveGraphSnapshot();
      const mapped = mapGraphToCanvas(payload);
      setFrames([]);
      setNodes(mapped.nodes);
      setEdges(mapped.edges);
      setLiveGraphSnapshot(payload);
      setSelectedNodeId(null);
      setConnectFromId(null);
      setDiffReport(null);
      setStatus(`Loaded ${mapped.nodes.length} nodes from Arch API.`);
    } catch (error) {
      const message = error instanceof Error ? error.message : String(error);
      setStatus(`Failed to load graph: ${message}`);
    }
  }

  async function connectAWSAndLoadGraph(): Promise<void> {
    if (trimmedAWSRegion === "") {
      setStatus("AWS region is required.");
      return;
    }
    if (trimmedAWSAccessKeyID === "") {
      setStatus("AWS access key ID is required.");
      return;
    }
    if (trimmedAWSSecretAccessKey === "") {
      setStatus("AWS secret access key is required.");
      return;
    }

    setIsConnectingAWS(true);
    try {
      const requestBody: AWSConnectRequest = {
        region: trimmedAWSRegion,
        access_key_id: trimmedAWSAccessKeyID,
        secret_access_key: trimmedAWSSecretAccessKey,
        validate_on_start: awsValidateOnConnect
      };

      if (trimmedAWSSessionToken !== "") {
        requestBody.session_token = trimmedAWSSessionToken;
      }
      if (trimmedAWSRoleARN !== "") {
        requestBody.role_arn = trimmedAWSRoleARN;
      }
      if (trimmedAWSSessionName !== "") {
        requestBody.session_name = trimmedAWSSessionName;
      }
      if (trimmedAWSExternalID !== "") {
        requestBody.external_id = trimmedAWSExternalID;
      }
      if (awsApplyTagFilters && trimmedStack !== "") {
        requestBody.stack_id = trimmedStack;
      }
      if (awsApplyTagFilters && trimmedEnvironment !== "") {
        requestBody.environment = trimmedEnvironment;
      }

      const response = await fetch("/api/v1/discovery/aws/graph", {
        method: "POST",
        headers: {
          "Content-Type": "application/json"
        },
        body: JSON.stringify(requestBody)
      });

      if (!response.ok) {
        throw new Error(await parseApiError(response));
      }

      const payload = (await response.json()) as AWSConnectResponse;
      const mapped = mapGraphToCanvas(payload.graph);
      setFrames([]);
      setNodes(mapped.nodes);
      setEdges(mapped.edges);
      setLiveGraphSnapshot(payload.graph);
      setDiffReport(null);
      setSelectedNodeId(null);
      setConnectFromId(null);
      setAwsConnection(payload);

      const account = payload.identity?.account_id || "unknown-account";
      setStatus(`Connected to AWS ${account} and loaded ${mapped.nodes.length} resources.`);
    } catch (error) {
      const message = error instanceof Error ? error.message : String(error);
      setStatus(`AWS connect failed: ${message}`);
    } finally {
      setIsConnectingAWS(false);
    }
  }

  async function generatePlan(): Promise<void> {
    if (nodeTagGuardrails.blocking.length > 0) {
      setStatus(`Plan blocked: ${nodeTagGuardrails.blocking[0]}`);
      return;
    }

    setIsPlanning(true);
    try {
      const before = liveGraphSnapshot || (await fetchLiveGraphSnapshot());
      const after = canvasToGraph(nodes, edges);
      const requestBody: {
        before: GraphSnapshot;
        after: GraphSnapshot;
        stack_id?: string;
        environment?: string;
      } = {
        before,
        after
      };

      if (trimmedStack !== "") {
        requestBody.stack_id = trimmedStack;
      }
      if (trimmedEnvironment !== "") {
        requestBody.environment = trimmedEnvironment;
      }

      const response = await fetch("/api/v1/graph/diff", {
        method: "POST",
        headers: {
          "Content-Type": "application/json"
        },
        body: JSON.stringify(requestBody)
      });

      if (!response.ok) {
        throw new Error(await parseApiError(response));
      }

      const report = (await response.json()) as GraphDiffReport;
      setLiveGraphSnapshot(before);
      setDiffReport(report);
      setStatus(`Plan ready: +${report.added} / ~${report.modified} / -${report.removed}.`);
    } catch (error) {
      const message = error instanceof Error ? error.message : String(error);
      setStatus(`Plan failed: ${message}`);
    } finally {
      setIsPlanning(false);
    }
  }

  function buildOperationRequest(): StackOperationRequest | null {
    const request: StackOperationRequest = {
      action: operationAction,
      stack_id: trimmedStack,
      environment: trimmedEnvironment,
      actor: trimmedActor,
      dry_run: dryRun
    };

    if (operationAction === "create" || operationAction === "scale") {
      const replicaCount = parsePositiveInt(replicas);
      if (replicaCount === null) {
        return null;
      }
      request.replicas = replicaCount;
    }

    if (operationAction === "create" || operationAction === "update") {
      request.tags = {
        "gocools:stack-id": trimmedStack,
        "gocools:environment": trimmedEnvironment,
        "gocools:owner": trimmedOwner
      };
      request.metadata = {
        source: "arch.frontend",
        node_count: String(nodes.length),
        edge_count: String(edges.length)
      };
    }

    if (operationAction === "destroy") {
      request.confirm = confirmDestroy;
      request.manual_override = manualOverride;
    }

    return request;
  }

  async function applyOperation(): Promise<void> {
    if (operationGuardrails.blocking.length > 0) {
      setStatus(`Operation blocked: ${operationGuardrails.blocking[0]}`);
      return;
    }

    const request = buildOperationRequest();
    if (!request) {
      setStatus("Operation blocked: invalid replicas value.");
      return;
    }

    setIsApplying(true);
    try {
      const response = await fetch("/api/v1/stacks/operations", {
        method: "POST",
        headers: {
          "Content-Type": "application/json"
        },
        body: JSON.stringify(request)
      });

      if (!response.ok) {
        throw new Error(await parseApiError(response));
      }

      const result = (await response.json()) as StackOperationResponse;
      setLastOperation(result);

      if (result.executed && !result.dry_run) {
        setLiveGraphSnapshot(canvasToGraph(nodes, edges));
        setDiffReport(null);
      }

      setStatus(
        `Operation ${request.action} ${result.executed ? "executed" : "validated"}: ${result.message}. Audit=${result.audit.result}.`
      );
    } catch (error) {
      const message = error instanceof Error ? error.message : String(error);
      setStatus(`Operation failed: ${message}`);
    } finally {
      setIsApplying(false);
    }
  }

  function stampRequiredTagsOnAllNodes(): void {
    if (nodes.length === 0) {
      setStatus("No nodes to tag.");
      return;
    }

    setNodes((current) =>
      current.map((node) => ({
        ...node,
        tags: {
          ...node.tags,
          "gocools:stack-id": trimmedStack,
          "gocools:environment": trimmedEnvironment,
          "gocools:owner": trimmedOwner
        }
      }))
    );
    setStatus(`Stamped required tags on ${nodes.length} nodes.`);
  }

  function exportJson(): void {
    const payload = {
      schema_version: "arch.frontend/v1alpha1",
      exported_at: new Date().toISOString(),
      frames,
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

  function onImportFile(event: ChangeEvent<HTMLInputElement>): void {
    const file = event.target.files?.[0];
    if (!file) {
      return;
    }

    const reader = new FileReader();
    reader.onload = () => {
      try {
        const parsed = JSON.parse(String(reader.result)) as {
          frames?: EditorFrame[];
          nodes?: EditorNode[];
          edges?: EditorEdge[];
        };
        if (!Array.isArray(parsed.nodes) || !Array.isArray(parsed.edges)) {
          throw new Error("invalid diagram payload");
        }

        const importedFrames = Array.isArray(parsed.frames) ? parsed.frames : [];
        const importedNodes = parsed.nodes.map((node) => ({
          ...node,
          width: node.width || NODE_WIDTH,
          height: node.height || NODE_HEIGHT,
          tags: normalizeRequiredTags(node.tags)
        }));

        setFrames(importedFrames);
        setNodes(importedNodes);
        setEdges(parsed.edges);
        setSelectedNodeId(null);
        setConnectFromId(null);
        setDiffReport(null);
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
          <p>AWS architecture editor and control plane for Arch.gocools</p>
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

          <h3>Cloud Connection</h3>
          <p className="subtle">
            Run ARCH on OCI and connect to AWS using temporary credentials or a role.
          </p>
          <label>
            AWS Region
            <input value={awsRegion} onChange={(event) => setAwsRegion(event.target.value)} />
          </label>
          <label>
            Access Key ID
            <input value={awsAccessKeyID} onChange={(event) => setAwsAccessKeyID(event.target.value)} />
          </label>
          <label>
            Secret Access Key
            <input
              type="password"
              value={awsSecretAccessKey}
              onChange={(event) => setAwsSecretAccessKey(event.target.value)}
            />
          </label>
          <label>
            Session Token (optional)
            <input
              type="password"
              value={awsSessionToken}
              onChange={(event) => setAwsSessionToken(event.target.value)}
            />
          </label>
          <label>
            Role ARN (optional)
            <input value={awsRoleARN} onChange={(event) => setAwsRoleARN(event.target.value)} />
          </label>
          <label>
            External ID (optional)
            <input value={awsExternalID} onChange={(event) => setAwsExternalID(event.target.value)} />
          </label>
          <label>
            Session Name (optional)
            <input value={awsSessionName} onChange={(event) => setAwsSessionName(event.target.value)} />
          </label>
          <label className="checkline">
            <input
              type="checkbox"
              checked={awsValidateOnConnect}
              onChange={(event) => setAwsValidateOnConnect(event.target.checked)}
            />
            Validate Credentials
          </label>
          <label className="checkline">
            <input
              type="checkbox"
              checked={awsApplyTagFilters}
              onChange={(event) => setAwsApplyTagFilters(event.target.checked)}
            />
            Apply Stack/Env Filters
          </label>
          <button className="solid" disabled={isConnectingAWS} onClick={connectAWSAndLoadGraph}>
            {isConnectingAWS ? "Connecting..." : "Connect AWS + Load Graph"}
          </button>
          {awsConnection ? (
            <div className="guard-ok">
              <p>
                Connected: {awsConnection.identity.account_id || "unknown-account"} ({awsConnection.region})
              </p>
              <p>{awsConnection.identity.arn || "identity ARN unavailable"}</p>
            </div>
          ) : null}

          <h3>Control Plane</h3>
          <label>
            Actor
            <input value={actor} onChange={(event) => setActor(event.target.value)} />
          </label>
          <label>
            Owner Tag
            <input value={operationOwner} onChange={(event) => setOperationOwner(event.target.value)} />
          </label>
          <label>
            Action
            <select
              value={operationAction}
              onChange={(event) => setOperationAction(event.target.value as OperationAction)}
            >
              <option value="create">create</option>
              <option value="update">update</option>
              <option value="scale">scale</option>
              <option value="destroy">destroy</option>
            </select>
          </label>
          {(operationAction === "create" || operationAction === "scale") && (
            <label>
              Replicas
              <input value={replicas} onChange={(event) => setReplicas(event.target.value)} inputMode="numeric" />
            </label>
          )}
          <label className="checkline">
            <input type="checkbox" checked={dryRun} onChange={(event) => setDryRun(event.target.checked)} />
            Dry Run
          </label>
          {operationAction === "destroy" && (
            <>
              <label className="checkline">
                <input
                  type="checkbox"
                  checked={confirmDestroy}
                  onChange={(event) => setConfirmDestroy(event.target.checked)}
                />
                Confirm Destroy
              </label>
              <label className="checkline">
                <input
                  type="checkbox"
                  checked={manualOverride}
                  onChange={(event) => setManualOverride(event.target.checked)}
                />
                Manual Override (prod)
              </label>
            </>
          )}

          <div className="action-grid">
            <button className="solid" disabled={isPlanning} onClick={generatePlan}>
              {isPlanning ? "Planning..." : "Generate Plan"}
            </button>
            <button
              className={operationAction === "destroy" ? "warn" : "solid"}
              disabled={isApplying}
              onClick={applyOperation}
            >
              {isApplying ? "Applying..." : "Apply Operation"}
            </button>
          </div>
          <button className="soft" onClick={stampRequiredTagsOnAllNodes}>
            Stamp Required Tags
          </button>

          <h3>Guardrails</h3>
          {operationGuardrails.blocking.length === 0 ? (
            <p className="guard-ok">No blocking guardrails.</p>
          ) : (
            <div className="guard-error">
              {operationGuardrails.blocking.slice(0, 5).map((item) => (
                <p key={item}>{item}</p>
              ))}
              {operationGuardrails.blocking.length > 5 ? (
                <p>+ {operationGuardrails.blocking.length - 5} more blockers.</p>
              ) : null}
            </div>
          )}
          {operationGuardrails.warnings.length > 0 ? (
            <div className="guard-warn">
              {operationGuardrails.warnings.slice(0, 5).map((item) => (
                <p key={item}>{item}</p>
              ))}
              {operationGuardrails.warnings.length > 5 ? (
                <p>+ {operationGuardrails.warnings.length - 5} more warnings.</p>
              ) : null}
            </div>
          ) : null}
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

              {frames.map((frame) => (
                <div
                  key={frame.id}
                  className={`canvas-frame ${frame.kind}`}
                  style={{
                    left: frame.x,
                    top: frame.y,
                    width: frame.width,
                    height: frame.height
                  }}
                >
                  <div className="frame-title">
                    <strong>{frame.title}</strong>
                    {frame.note ? <small>{frame.note}</small> : null}
                  </div>
                </div>
              ))}

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
                  <div className="node-heading">
                    <span className="node-icon">{typeIcon(node.type)}</span>
                    <strong>{node.title}</strong>
                  </div>
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
                <input value={selectedNode.type} onChange={(event) => updateSelectedNode({ type: event.target.value })} />
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
                <input value={selectedNode.state} onChange={(event) => updateSelectedNode({ state: event.target.value })} />
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
            <p>Frames: {frames.length}</p>
            <p>Nodes: {nodes.length}</p>
            <p>Edges: {edges.length}</p>
            <p>Mode: {mode}</p>
            <p>Baseline: {liveGraphSnapshot ? "loaded" : "not loaded"}</p>
            {connectFromId ? <p>Connecting from: {connectFromId}</p> : null}
          </div>

          <div className="meta-block">
            <h3>Plan Preview</h3>
            {!diffReport ? (
              <p>No plan generated yet.</p>
            ) : (
              <>
                <p>
                  Added: {diffReport.added} | Modified: {diffReport.modified} | Removed: {diffReport.removed}
                </p>
                <div className="diff-list">
                  {diffReport.changes.slice(0, 8).map((change) => (
                    <div className="diff-item" key={`${change.kind}-${change.node_id}`}>
                      <span className={`diff-kind ${change.kind}`}>{change.kind}</span>
                      <strong>{change.node_id}</strong>
                      <small>{change.resource_type}</small>
                      <p>{summarizeDiff(change)}</p>
                    </div>
                  ))}
                </div>
                {diffReport.changes.length > 8 ? <p>Showing first 8 of {diffReport.changes.length} changes.</p> : null}
              </>
            )}
          </div>

          <div className="meta-block">
            <h3>Last Operation</h3>
            {!lastOperation ? (
              <p>No stack operation submitted yet.</p>
            ) : (
              <>
                <p>{lastOperation.message}</p>
                <p>Executed: {lastOperation.executed ? "yes" : "no"}</p>
                <p>Dry run: {lastOperation.dry_run ? "yes" : "no"}</p>
                <p>
                  Audit: {lastOperation.audit.action} by {lastOperation.audit.actor}
                </p>
                <p>{new Date(lastOperation.audit.timestamp).toLocaleString()}</p>
              </>
            )}
          </div>

          <p className="powered-by">Interaction model inspired by edit.gocools.</p>
        </aside>
      </div>
    </div>
  );
}
