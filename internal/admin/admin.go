package admin

import (
	"encoding/json"
	"html/template"
	"log/slog"
	"net/http"
	"os"

	"github.com/aegisproxy/core/internal/sanitizer"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type AdminServer struct {
	regexExt *sanitizer.RegexExtractor
}

func NewAdminServer(regexExt *sanitizer.RegexExtractor) *AdminServer {
	return &AdminServer{
		regexExt: regexExt,
	}
}

func (s *AdminServer) Start(port string) {
	mux := http.NewServeMux()
	
	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/api/rules", s.handleRules)
	mux.HandleFunc("/", s.handleUI)

	slog.Info("Starting Admin/Metrics server", "port", port)
	if err := http.ListenAndServe(port, mux); err != nil {
		slog.Error("Admin server failed", "error", err)
		os.Exit(1)
	}
}

func (s *AdminServer) handleRules(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		rules := s.regexExt.GetRules()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(rules)
		return
	}

	if r.Method == http.MethodPost {
		var req struct {
			Type    string `json:"type"`
			Pattern string `json:"pattern"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		
		if err := s.regexExt.AddRule(req.Type, req.Pattern); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		
		slog.Info("Added new rule via Admin UI", "type", req.Type)
		w.WriteHeader(http.StatusCreated)
		return
	}
	
	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

func (s *AdminServer) handleUI(w http.ResponseWriter, r *http.Request) {
	tmpl := `
<!DOCTYPE html>
<html>
<head>
	<title>AegisProxy Admin</title>
	<style>
		body { font-family: sans-serif; padding: 20px; }
		table { border-collapse: collapse; width: 100%; margin-bottom: 20px; }
		th, td { border: 1px solid #ddd; padding: 8px; text-align: left; }
		th { background-color: #f2f2f2; }
		form { margin-top: 20px; border: 1px solid #ddd; padding: 15px; width: 400px; }
		input { margin-bottom: 10px; width: 100%; padding: 5px; }
		button { padding: 10px; background: #007bff; color: white; border: none; cursor: pointer; }
	</style>
</head>
<body>
	<h1>AegisProxy Admin Dashboard</h1>
	<h2>Current Rules</h2>
	<table id="rulesTable">
		<tr><th>Type</th><th>Regex Pattern</th></tr>
	</table>

	<h2>Add New Rule (In-Memory)</h2>
	<form id="addRuleForm">
		<label>Type (e.g. PASSPORT)</label>
		<input type="text" id="ruleType" required>
		<label>Regex Pattern (e.g. \d{4} \d{6})</label>
		<input type="text" id="rulePattern" required>
		<button type="submit">Add Rule</button>
	</form>

	<script>
		async function fetchRules() {
			const res = await fetch('/api/rules');
			const rules = await res.json();
			const table = document.getElementById('rulesTable');
			table.innerHTML = '<tr><th>Type</th><th>Regex Pattern</th></tr>';
			rules.forEach(r => {
				table.innerHTML += '<tr><td>'+r.type+'</td><td>'+r.pattern+'</td></tr>';
			});
		}
		
		document.getElementById('addRuleForm').addEventListener('submit', async (e) => {
			e.preventDefault();
			const type = document.getElementById('ruleType').value;
			const pattern = document.getElementById('rulePattern').value;
			const res = await fetch('/api/rules', {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({type, pattern})
			});
			if (res.ok) {
				fetchRules();
				document.getElementById('addRuleForm').reset();
			} else {
				alert('Failed to add rule');
			}
		});

		fetchRules();
	</script>
</body>
</html>`
	
	t, _ := template.New("ui").Parse(tmpl)
	_ = t.Execute(w, nil)
}
