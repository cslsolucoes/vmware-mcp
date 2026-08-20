# -*- coding: utf-8 -*-
"""Gera TOOLS.md — a referencia completa das tools MCP (descricao + sintaxe).

Fonte: o dump do proprio servidor (src/cmd/dumptools), que emite name +
description + inputSchema por modo de conexao. Nao editar TOOLS.md a mao.

Uso:
    cd src && go run ./cmd/dumptools > catalog.json
    python scripts/gen_tools_md.py [catalog.json] [TOOLS.md]
"""
import json, io, sys, os

CATALOG = sys.argv[1] if len(sys.argv) > 1 else "catalog.json"
OUT = sys.argv[2] if len(sys.argv) > 2 else "TOOLS.md"


def fix(s):
    """Repara mojibake 'UTF-8 lido como CP1252' (ex.: em-dash), sem
    corromper strings ja corretas."""
    if not s or ("â" not in s and "Â" not in s):
        return s
    try:
        return s.encode("cp1252").decode("utf-8")
    except (UnicodeEncodeError, UnicodeDecodeError):
        return s


def esc(s):
    return fix(s).replace("\r", " ").replace("\n", " ").replace("|", "\\|").strip()


cat = json.load(open(CATALOG, encoding="utf-8"))
byname = {t["name"]: dict(t) for t in cat["vcenter"]}
vm = {t["name"] for t in cat["vmware"]}
ws = {t["name"]: t for t in cat["workstation"]}
aws = {t["name"]: t for t in cat["cloudaws"]}


def mode_of(name):
    if name in ws:
        return "workstation"
    if name in aws:
        return "cloud-aws"
    return "vsphere-general" if name in vm else "vcenter-only"


allt = {}
allt.update(byname)
allt.update(ws)
allt.update(aws)

DOMAIN_TITLE = {
    "about": "Sistema", "authentication": "Autenticacao", "authorization": "Autorizacao (papeis/permissoes)",
    "alarm": "Alarmes", "cis": "CIS / Tasks", "cluster": "Cluster (modulos/DRS)", "compute": "Compute Resource",
    "crypto": "Criptografia / KMS", "custom": "Custom Fields", "customization": "Customization Spec",
    "datacenter": "Datacenter", "datastore": "Datastore (inventario)", "diagnostic": "Diagnostico / Logs", "dvpg": "DVS - Port Group",
    "dvs": "DVS (Distributed Virtual Switch)", "dvsmgr": "DVS - Manager", "environment": "Environment Browser",
    "esx": "ESX Settings (cluster VMs)", "event": "Eventos", "extension": "Extension Manager", "fcd": "First-Class Disks (vCenter)",
    "fcdhost": "First-Class Disks (host)", "file": "Datastore - Arquivos",
    "folder": "Inventario - Folder", "guest": "Guest Operations (SO convidado)", "health": "Health Update Manager",
    "host": "Host (ESXi)", "iofilter": "IO Filter", "ippool": "IP Pool", "library": "Content Library", "license": "Licencas",
    "list": "Inventario - Listagens", "namespace": "Supervisor / Namespaces (Tanzu)", "opaque": "Rede opaca (NSX)",
    "perf": "Performance (metricas)", "resource": "Resource Pool / vApp", "scheduledtask": "Scheduled Tasks",
    "search": "Search Index", "storage": "Storage DRS / SDRS", "tags": "Tags & Categorias", "task": "Tasks",
    "tenant": "Multi-tenant (service provider)", "vapp": "vApp", "vcenter": "vCenter - OVF/Template/Library",
    "virtual": "Virtual Disk (VDDK)", "vm": "Virtual Machine", "cloudaws": "VMware Cloud on AWS",
    "workstation": "Workstation Pro", "appliance": "VAMI (vCenter Server Appliance)",
}
BIG = {"host", "appliance", "cloudaws"}
BIGP = {"host": "Host", "appliance": "VAMI", "cloudaws": "VMC"}
SPECIAL = {"iscsi": "iSCSI", "dns": "DNS", "ntp": "NTP", "snmp": "SNMP", "ipv4": "IPv4", "ipv6": "IPv6",
    "vsan": "vSAN", "nic": "NIC", "ip": "IP", "vmon": "vMon", "dcui": "DCUI", "ssh": "SSH",
    "vflash": "vFlash", "sddc": "SDDC", "dataset": "DataSet", "vmnet": "VMnet"}


def domain(name):
    p = name.split("_")
    tok = p[1] if len(p) > 1 else "outros"
    if tok in BIG and len(p) > 2:
        return tok + "_" + p[2]
    return tok


def dtitle(d):
    if "_" in d:
        a, b = d.split("_", 1)
        return "%s - %s" % (BIGP.get(a, a.title()), SPECIAL.get(b, b.replace("-", " ").title()))
    return DOMAIN_TITLE.get(d, d.title())


def product(name):
    if name in ws:
        return ("C", "VMware Workstation Pro")
    if name in aws:
        return ("B", "VMware Cloud on AWS (VMC)")
    return ("A", "vSphere / vCenter / ESXi")


tree = {}
for name in allt:
    pk, pl = product(name)
    tree.setdefault((pk, pl), {}).setdefault(domain(name), []).append(name)


def sig(name):
    sch = allt[name].get("inputSchema") or {}
    props = sch.get("properties") or {}
    req = set(sch.get("required") or [])
    reqs = [p for p in sorted(props) if p in req]
    opts = [p for p in sorted(props) if p not in req]
    inside = ", ".join(reqs)
    if opts:
        inside += (", " if reqs else "") + "[" + ", ".join(opts) + "]"
    return "%s(%s)" % (name, inside)


def typestr(d):
    t = d.get("type", "")
    if t == "array":
        return "array&lt;%s&gt;" % ((d.get("items") or {}).get("type", "object"))
    return t or "object"


buf = io.StringIO()
w = buf.write
total = len(allt)
w("# Referencia completa de ferramentas — MCPVMWare\n\n")
w("Este documento lista **as %d ferramentas MCP** do servidor MCPVMWare, cada uma com **descricao** e **sintaxe** (parametros de entrada). E gerado a partir do proprio catalogo do servidor (`tools/list`).\n\n" % total)
w("> Gerado por `src/cmd/dumptools` + `scripts/gen_tools_md.py`. Nao editar a mao — regenerar apos mudar tools.\n\n")
w("Legenda de modo: `vcenter-only` (so com `--vcenter-url`/`--vmware-all-url`) · `vsphere-general` (tambem em ESXi standalone) · `cloud-aws` · `workstation`. Ferramentas *(destrutiva)* exigem `--allow-destructive` + `confirm:true`.\n\n")

w("## Indice\n\n")
for (pk, pl) in sorted(tree):
    doms = tree[(pk, pl)]
    w("- **%s** (%d)\n" % (pl, sum(len(v) for v in doms.values())))
    for d in sorted(doms):
        w("  - %s — %d\n" % (dtitle(d), len(doms[d])))
w("\n---\n\n")

for (pk, pl) in sorted(tree):
    doms = tree[(pk, pl)]
    w("## %s (%d)\n\n" % (pl, sum(len(v) for v in doms.values())))
    for d in sorted(doms):
        names = sorted(doms[d])
        w("### %s (%d)\n\n" % (dtitle(d), len(names)))
        for name in names:
            t = allt[name]
            props = (t.get("inputSchema") or {}).get("properties") or {}
            req = set((t.get("inputSchema") or {}).get("required") or [])
            tag = "  *(destrutiva)*" if "confirm" in props else ""
            w("#### `%s`\n\n" % name)
            w("**Modo:** `%s`%s\n\n" % (mode_of(name), tag))
            w("%s\n\n" % fix(t.get("description", "")).strip())
            w("**Sintaxe:** `%s`\n\n" % sig(name))
            if props:
                w("| Parametro | Tipo | Obrig. | Descricao |\n|---|---|:--:|---|\n")
                for p in sorted(props):
                    w("| `%s` | %s | %s | %s |\n" % (p, typestr(props[p]), "sim" if p in req else "—", esc(props[p].get("description", ""))))
                w("\n")
            else:
                w("_Sem parametros._\n\n")
    w("---\n\n")

open(OUT, "w", encoding="utf-8", newline="\n").write(buf.getvalue())
print("TOOLS.md escrito: %s (%d tools, %d bytes)" % (OUT, total, len(buf.getvalue())))
