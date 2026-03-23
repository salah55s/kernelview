import { BrowserRouter, Routes, Route, NavLink, Navigate } from 'react-router-dom';
import { Map, Activity, AlertTriangle, TrendingDown, Moon, Sun } from 'lucide-react';
import { useState, useEffect } from 'react';
import ServiceMap from '@/pages/ServiceMap';
import ServiceDetail from '@/pages/ServiceDetail';
import IncidentDetail from '@/pages/IncidentDetail';
import RightSizing from '@/pages/RightSizing';

function App() {
  const [dark, setDark] = useState(true);

  useEffect(() => {
    document.documentElement.classList.toggle('dark', dark);
  }, [dark]);

  return (
    <BrowserRouter>
      <div className="min-h-screen bg-background">
        {/* Top navigation */}
        <header className="sticky top-0 z-50 border-b bg-background/80 backdrop-blur-xl">
          <div className="flex h-14 items-center px-6">
            {/* Logo */}
            <div className="flex items-center gap-2 mr-8">
              <img src="/logo.png" alt="KernelView Logo" className="h-8 w-8 rounded-lg object-cover" />
              <span className="text-lg font-bold tracking-tight">
                Kernel<span className="text-blue-500">View</span>
              </span>
            </div>

            {/* Nav links */}
            <nav className="flex items-center gap-1">
              <NavLink
                to="/map"
                className={({ isActive }) =>
                  `flex items-center gap-2 px-3 py-1.5 rounded-md text-sm font-medium transition-colors ${
                    isActive
                      ? 'bg-secondary text-foreground'
                      : 'text-muted-foreground hover:text-foreground hover:bg-secondary/50'
                  }`
                }
              >
                <Map className="h-4 w-4" />
                Service Map
              </NavLink>
              <NavLink
                to="/incidents"
                className={({ isActive }) =>
                  `flex items-center gap-2 px-3 py-1.5 rounded-md text-sm font-medium transition-colors ${
                    isActive
                      ? 'bg-secondary text-foreground'
                      : 'text-muted-foreground hover:text-foreground hover:bg-secondary/50'
                  }`
                }
              >
                <AlertTriangle className="h-4 w-4" />
                Incidents
              </NavLink>
              <NavLink
                to="/right-sizing"
                className={({ isActive }) =>
                  `flex items-center gap-2 px-3 py-1.5 rounded-md text-sm font-medium transition-colors ${
                    isActive
                      ? 'bg-secondary text-foreground'
                      : 'text-muted-foreground hover:text-foreground hover:bg-secondary/50'
                  }`
                }
              >
                <TrendingDown className="h-4 w-4" />
                Right-Sizing
              </NavLink>
            </nav>

            {/* Right side */}
            <div className="ml-auto flex items-center gap-3">
              <div className="flex items-center gap-2 text-sm text-muted-foreground">
                <Activity className="h-4 w-4 text-emerald-500" />
                <span>Connected</span>
              </div>
              <button
                onClick={() => setDark(!dark)}
                className="rounded-md p-2 hover:bg-secondary transition-colors"
              >
                {dark ? <Sun className="h-4 w-4" /> : <Moon className="h-4 w-4" />}
              </button>
            </div>
          </div>
        </header>

        {/* Page content */}
        <main className="p-6">
          <Routes>
            <Route path="/" element={<Navigate to="/map" replace />} />
            <Route path="/map" element={<ServiceMap />} />
            <Route path="/services/:service" element={<ServiceDetail />} />
            <Route path="/incidents" element={<IncidentDetail />} />
            <Route path="/incidents/:id" element={<IncidentDetail />} />
            <Route path="/right-sizing" element={<RightSizing />} />
          </Routes>
        </main>
      </div>
    </BrowserRouter>
  );
}

export default App;
