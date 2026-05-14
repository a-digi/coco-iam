import { BrowserRouter, Routes, Route } from 'react-router-dom';
import Layout from './Layout';
import { AuthProvider } from './Components/Auth/Guard/AuthContext';
import { AdminProfileProvider } from './Components/Auth/Profile/AdminProfileProvider';
import { routes } from './config/routing/routes.tsx';
import { MenuProvider } from './Components/Menu/MenuProvider.tsx';
import { HttpClientProvider } from './api/http/HttpClient.tsx';
import { SnackBarProvider } from './Shared/Components/SnackBar/SnackBarProvider.tsx';

function App() {
  return (
    <HttpClientProvider>
      <SnackBarProvider>
        <AuthProvider>
          <AdminProfileProvider>
            <MenuProvider>
              <BrowserRouter>
                <Layout>
                  <Routes>
                    {routes.map(({ path, component: Component, element, children }) =>
                      Component ? (
                        <Route key={path} path={path} element={<Component />}>
                          {children && children.map(child =>
                            child.component ? (
                              <Route key={child.path} path={child.path} element={<child.component />} />
                            ) : child.element ? (
                              <Route key={child.path} path={child.path} element={child.element} />
                            ) : null
                          )}
                        </Route>
                      ) : element ? (
                        <Route key={path} path={path} element={element} />
                      ) : null
                    )}
                  </Routes>
                </Layout>
              </BrowserRouter>
            </MenuProvider>
          </AdminProfileProvider>
        </AuthProvider>
      </SnackBarProvider>
    </HttpClientProvider>
  )
}

export default App
